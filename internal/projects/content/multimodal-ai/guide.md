# Multimodal AI: Text + Image Understanding Pipeline

## Overview

Multimodal AI systems process and reason over multiple data types simultaneously — text, images, audio, and video. The most impactful applications combine vision and language: image captioning, visual question answering (VQA), document understanding, and content moderation. These capabilities power accessibility tools, document processing automation, visual search, and content analysis at scale.

This project builds a multimodal pipeline handling text and images: generating captions from images (BLIP-2), answering questions about images (LLaVA), extracting text from documents (OCR + layout analysis), and combining visual and textual features for downstream tasks. These skills are increasingly essential as AI systems move beyond text-only processing.

## Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                   Multimodal Pipeline                            │
│                                                                  │
│  ┌──────────┐   ┌──────────────┐   ┌──────────────┐            │
│  │ Image    │──▶│ Vision       │──▶│ Caption /    │            │
│  │ Input    │   │ Encoder      │   │ VQA Output   │            │
│  └──────────┘   │ (ViT/CLIP)  │   └──────────────┘            │
│                  └──────┬───────┘                                │
│                         │                                        │
│  ┌──────────┐   ┌──────▼───────┐   ┌──────────────┐            │
│  │ Text     │──▶│ Multimodal   │──▶│ Unified      │            │
│  │ Input    │   │ Fusion       │   │ Response     │            │
│  └──────────┘   │ (Q-Former)   │   └──────────────┘            │
│                  └──────────────┘                                │
│                                                                  │
│  ┌──────────┐   ┌──────────────┐   ┌──────────────┐            │
│  │ Document │──▶│ OCR +        │──▶│ Structured   │            │
│  │ Image    │   │ Layout       │   │ Extraction   │            │
│  └──────────┘   │ (Tesseract)  │   └──────────────┘            │
│                  └──────────────┘                                │
└──────────────────────────────────────────────────────────────────┘
```

**Components:**

- **Vision Encoder** — Converts images into dense feature representations. Uses Vision Transformer (ViT) or CLIP encoder. Processes at multiple resolutions for detail preservation.
- **Multimodal Fusion** — Bridges vision and language representations. BLIP-2 uses a Q-Former (Querying Transformer) to extract language-relevant visual features. LLaVA uses a simple linear projection.
- **Caption / VQA Output** — Generates natural language descriptions of images or answers visual questions grounded in image content.
- **OCR + Layout Analysis** — Extracts text from document images with spatial information. Combines Tesseract OCR with layout understanding for forms, invoices, and receipts.
- **Structured Extraction** — Converts OCR output into structured data (JSON) for downstream processing.

## Key Concepts

### Vision-Language Models

Modern vision-language models follow a common pattern: a pre-trained vision encoder (ViT) extracts visual features, a bridging module aligns visual and language representations, and a language model generates text conditioned on the visual features.

**BLIP-2** (Salesforce) uses a lightweight Q-Former between a frozen image encoder and a frozen LLM. The Q-Former learns to extract the most informative visual features through cross-attention with learnable query tokens. This is parameter-efficient — only the Q-Former is trained.

**LLaVA** (Large Language and Vision Assistant) takes a simpler approach: a linear projection maps ViT features directly into the LLM's embedding space. Despite its simplicity, LLaVA achieves competitive performance by leveraging instruction-tuned LLMs and high-quality visual instruction tuning data.

**CLIP** (OpenAI) learns aligned image-text embeddings via contrastive learning. It doesn't generate text but excels at zero-shot classification ("is this image a cat or a dog?") and image-text similarity ranking.

### Image Preprocessing

Vision models are sensitive to preprocessing:

- **Resolution**: Most models expect 224x224 or 336x336 pixels. Higher resolution preserves detail but increases computation quadratically (ViT processes images as patches).
- **Normalization**: Match the training distribution — ImageNet normalization (mean=[0.485, 0.456, 0.406], std=[0.229, 0.224, 0.225]) is standard for most ViTs.
- **Aspect ratio**: Resizing to a square distorts images. Center-crop or pad to maintain aspect ratio. For document understanding, preserving aspect ratio is critical for text layout.

### Document Understanding

Document AI combines OCR (reading text from images) with layout analysis (understanding spatial relationships). A receipt has items in columns; a form has labels and values in spatial alignment. Pure OCR outputs a flat text string — layout analysis preserves the 2D structure.

**Tesseract** is the standard open-source OCR engine. Version 5 uses LSTM-based recognition and handles most printed text well. For handwriting or degraded documents, cloud APIs (Google Cloud Vision, AWS Textract) are more accurate.

**Layout analysis** identifies regions (text blocks, tables, figures) and their relationships. LayoutLM (Microsoft) processes tokens with their bounding box coordinates, combining text and spatial information in a single transformer. This enables extraction of key-value pairs from forms without custom rules.

## Implementation Steps

### Step 1: Environment Setup

```python
# requirements.txt
transformers==4.44.2
torch==2.4.1
Pillow==10.4.0
pytesseract==0.3.13
accelerate==0.34.2
bitsandbytes==0.43.3
```

System dependency:
```bash
sudo apt install tesseract-ocr
```

### Step 2: Image Captioning with BLIP-2

```python
# captioning.py
import torch
from transformers import Blip2Processor, Blip2ForConditionalGeneration
from PIL import Image
from pathlib import Path

class ImageCaptioner:
    def __init__(self, model_name: str = "Salesforce/blip2-opt-2.7b",
                 device: str = None):
        self.device = device or ("cuda" if torch.cuda.is_available() else "cpu")
        self.processor = Blip2Processor.from_pretrained(model_name)
        self.model = Blip2ForConditionalGeneration.from_pretrained(
            model_name,
            torch_dtype=torch.float16 if self.device == "cuda" else torch.float32,
            device_map="auto" if self.device == "cuda" else None,
        )
        if self.device == "cpu":
            self.model = self.model.to(self.device)

    def caption(self, image_path: str, prompt: str = None) -> str:
        """Generate a caption for an image.

        Args:
            image_path: Path to the image file.
            prompt: Optional prompt to guide captioning
                    (e.g., "a photograph of").
        """
        image = Image.open(image_path).convert("RGB")

        if prompt:
            inputs = self.processor(
                images=image, text=prompt, return_tensors="pt"
            ).to(self.device, torch.float16 if self.device == "cuda"
                 else torch.float32)
        else:
            inputs = self.processor(
                images=image, return_tensors="pt"
            ).to(self.device, torch.float16 if self.device == "cuda"
                 else torch.float32)

        with torch.no_grad():
            generated_ids = self.model.generate(
                **inputs,
                max_new_tokens=100,
                num_beams=5,
                early_stopping=True,
            )

        caption = self.processor.batch_decode(
            generated_ids, skip_special_tokens=True
        )[0].strip()
        return caption

    def caption_batch(self, image_paths: list[str],
                      prompt: str = None) -> list[str]:
        """Generate captions for multiple images."""
        images = [Image.open(p).convert("RGB") for p in image_paths]

        if prompt:
            inputs = self.processor(
                images=images, text=[prompt] * len(images),
                return_tensors="pt", padding=True
            ).to(self.device)
        else:
            inputs = self.processor(
                images=images, return_tensors="pt", padding=True
            ).to(self.device)

        with torch.no_grad():
            generated_ids = self.model.generate(**inputs, max_new_tokens=100)

        captions = self.processor.batch_decode(
            generated_ids, skip_special_tokens=True
        )
        return [c.strip() for c in captions]
```

### Step 3: Visual Question Answering

```python
# vqa.py
from transformers import LlavaNextProcessor, LlavaNextForConditionalGeneration
import torch
from PIL import Image

class VisualQA:
    def __init__(self, model_name: str = "llava-hf/llava-v1.6-mistral-7b-hf"):
        self.device = "cuda" if torch.cuda.is_available() else "cpu"
        self.processor = LlavaNextProcessor.from_pretrained(model_name)
        self.model = LlavaNextForConditionalGeneration.from_pretrained(
            model_name,
            torch_dtype=torch.float16,
            device_map="auto",
            load_in_4bit=True,  # QLoRA for memory efficiency
        )

    def ask(self, image_path: str, question: str) -> str:
        """Ask a question about an image."""
        image = Image.open(image_path).convert("RGB")

        # LLaVA uses a specific conversation format
        conversation = [
            {
                "role": "user",
                "content": [
                    {"type": "image"},
                    {"type": "text", "text": question},
                ],
            },
        ]

        prompt = self.processor.apply_chat_template(
            conversation, add_generation_prompt=True
        )
        inputs = self.processor(
            images=image, text=prompt, return_tensors="pt"
        ).to(self.device)

        with torch.no_grad():
            output = self.model.generate(
                **inputs,
                max_new_tokens=512,
                do_sample=False,
            )

        # Decode only the generated tokens
        answer = self.processor.decode(
            output[0][inputs["input_ids"].shape[1]:],
            skip_special_tokens=True,
        )
        return answer.strip()

    def analyze(self, image_path: str) -> dict:
        """Comprehensive image analysis with multiple questions."""
        questions = [
            "Describe this image in detail.",
            "What objects are visible in this image?",
            "What is the setting or location shown?",
            "What text, if any, appears in this image?",
        ]
        results = {}
        for q in questions:
            results[q] = self.ask(image_path, q)
        return results
```

### Step 4: Document OCR and Extraction

```python
# document.py
import pytesseract
from PIL import Image
import json
from dataclasses import dataclass

@dataclass
class TextBlock:
    text: str
    x: int
    y: int
    width: int
    height: int
    confidence: float

class DocumentExtractor:
    def __init__(self, lang: str = "eng"):
        self.lang = lang

    def extract_text(self, image_path: str) -> str:
        """Extract plain text from a document image."""
        image = Image.open(image_path)
        text = pytesseract.image_to_string(image, lang=self.lang)
        return text.strip()

    def extract_blocks(self, image_path: str,
                       min_confidence: float = 60.0) -> list[TextBlock]:
        """Extract text with bounding box positions."""
        image = Image.open(image_path)
        data = pytesseract.image_to_data(
            image, lang=self.lang, output_type=pytesseract.Output.DICT
        )

        blocks = []
        n_boxes = len(data["text"])
        for i in range(n_boxes):
            conf = float(data["conf"][i])
            text = data["text"][i].strip()
            if conf >= min_confidence and text:
                blocks.append(TextBlock(
                    text=text,
                    x=data["left"][i],
                    y=data["top"][i],
                    width=data["width"][i],
                    height=data["height"][i],
                    confidence=conf,
                ))
        return blocks

    def extract_table(self, image_path: str) -> list[list[str]]:
        """Extract tabular data from a document image."""
        blocks = self.extract_blocks(image_path)
        if not blocks:
            return []

        # Group blocks into rows by Y-coordinate proximity
        blocks.sort(key=lambda b: b.y)
        rows = []
        current_row = [blocks[0]]
        row_y = blocks[0].y

        for block in blocks[1:]:
            if abs(block.y - row_y) < 15:  # Same row threshold
                current_row.append(block)
            else:
                current_row.sort(key=lambda b: b.x)
                rows.append([b.text for b in current_row])
                current_row = [block]
                row_y = block.y

        if current_row:
            current_row.sort(key=lambda b: b.x)
            rows.append([b.text for b in current_row])

        return rows

    def extract_key_value_pairs(self, image_path: str) -> dict:
        """Extract key-value pairs from forms and documents."""
        blocks = self.extract_blocks(image_path)

        # Heuristic: label is left of or above its value
        pairs = {}
        for i, block in enumerate(blocks):
            text = block.text.rstrip(":")
            # Check if this looks like a label (ends with colon, short, etc.)
            if block.text.endswith(":") or block.text.endswith("="):
                # Find the nearest block to the right
                candidates = [
                    b for b in blocks
                    if b.x > block.x + block.width
                    and abs(b.y - block.y) < 15
                ]
                if candidates:
                    candidates.sort(key=lambda b: b.x)
                    pairs[text] = candidates[0].text

        return pairs
```

### Step 5: Unified Multimodal Pipeline

```python
# pipeline.py
from dataclasses import dataclass
from pathlib import Path

@dataclass
class MultimodalResult:
    caption: str = ""
    vqa_answers: dict = None
    ocr_text: str = ""
    extracted_data: dict = None
    image_type: str = ""  # photo, document, chart, screenshot

class MultimodalPipeline:
    def __init__(self):
        self.captioner = ImageCaptioner()
        self.vqa = VisualQA()
        self.doc_extractor = DocumentExtractor()

    def classify_image(self, image_path: str) -> str:
        """Determine the type of image for routing."""
        answer = self.vqa.ask(
            image_path,
            "Is this image a photograph, a document/form, a chart/graph, "
            "or a screenshot? Reply with one word."
        )
        answer = answer.lower()
        if "document" in answer or "form" in answer:
            return "document"
        elif "chart" in answer or "graph" in answer:
            return "chart"
        elif "screenshot" in answer:
            return "screenshot"
        return "photo"

    def process(self, image_path: str,
                questions: list[str] = None) -> MultimodalResult:
        """Process an image through the appropriate pipeline."""
        result = MultimodalResult()

        # Classify image type
        result.image_type = self.classify_image(image_path)

        # Always generate a caption
        result.caption = self.captioner.caption(image_path)

        # Route based on image type
        if result.image_type == "document":
            result.ocr_text = self.doc_extractor.extract_text(image_path)
            result.extracted_data = (
                self.doc_extractor.extract_key_value_pairs(image_path)
            )
        elif result.image_type == "chart":
            result.vqa_answers = {
                "What does this chart show?": self.vqa.ask(
                    image_path, "What does this chart show?"
                ),
                "What are the key trends?": self.vqa.ask(
                    image_path, "What are the key data points or trends?"
                ),
            }

        # Answer custom questions
        if questions:
            result.vqa_answers = result.vqa_answers or {}
            for q in questions:
                result.vqa_answers[q] = self.vqa.ask(image_path, q)

        return result
```

## Testing & Measurement

### Quality Metrics

- **Captioning**: BLEU, METEOR, CIDEr scores against reference captions. Use COCO Captions validation set for benchmarking.
- **VQA**: Accuracy on VQA v2.0 benchmark. For custom evaluation, human scoring on a 1-5 scale for relevance and accuracy.
- **OCR**: Character Error Rate (CER) and Word Error Rate (WER) against ground truth transcriptions. Target CER < 5% for printed text.
- **Document extraction**: Field-level precision and recall for key-value pairs against manually annotated documents.

### Testing Strategy

```python
def test_captioning_quality():
    captioner = ImageCaptioner()
    # Test with a known image
    caption = captioner.caption("test_images/cat_on_couch.jpg")
    assert "cat" in caption.lower(), f"Expected 'cat' in caption: {caption}"
    assert len(caption.split()) > 3, "Caption too short"

def test_ocr_accuracy():
    extractor = DocumentExtractor()
    text = extractor.extract_text("test_images/sample_invoice.png")
    assert "Invoice" in text or "INVOICE" in text
    assert "$" in text  # Should contain dollar amounts

def test_vqa_grounding():
    vqa = VisualQA()
    # Image shows a red car
    answer = vqa.ask("test_images/red_car.jpg", "What color is the car?")
    assert "red" in answer.lower()
```

## Interview Angles

### Q1: How do vision-language models bridge the gap between image and text representations?

**Sample Answer:** The core challenge is that images and text live in different representation spaces. Three main approaches exist. CLIP-style contrastive learning trains separate image and text encoders to produce aligned embeddings — similar image-text pairs have high cosine similarity. This is great for retrieval and classification but can't generate text. BLIP-2 uses a Q-Former, a small transformer that cross-attends to frozen visual features with learnable queries, extracting the most language-relevant visual information. This is parameter-efficient because only the Q-Former trains. LLaVA takes the simplest approach — a linear projection maps visual tokens directly into the LLM's token space, treating image features as a "visual prefix" to the text prompt. The tradeoff is between representation quality and training cost. BLIP-2's Q-Former produces richer multimodal representations but requires more sophisticated training. LLaVA's linear projection is simpler to train but relies more heavily on the LLM's ability to interpret raw visual features.

### Q2: How do you handle documents with complex layouts (tables, multi-column, forms)?

**Sample Answer:** Pure OCR produces a flat text string that loses spatial relationships. I use a three-step approach. Step 1: Layout analysis to segment the page into regions (text blocks, tables, figures, headers). LayoutLMv3 or YOLO-based detectors identify these regions with bounding boxes. Step 2: Within each region, apply specialized extraction — OCR for text blocks, table structure recognition for tables (Microsoft's Table Transformer detects rows and columns), and the multimodal model for figures and charts. Step 3: Reconstruct the logical reading order and structure. For multi-column layouts, sort regions left-to-right within each vertical zone. For forms, pair labels with their values using spatial proximity. The tradeoff is between rule-based and ML approaches — rules are interpretable and work for standardized document formats, but ML generalizes better to unseen layouts. I start with rules for known document types and fall back to ML for novel formats.

### Q3: What are the main challenges in deploying multimodal models to production?

**Sample Answer:** Three main challenges. (1) Compute cost: multimodal models are significantly larger than text-only models. BLIP-2 with OPT-6.7B needs ~14GB VRAM. For production, I use quantization (4-bit with bitsandbytes) and model distillation. A common pattern is to use the large model to generate training data, then fine-tune a smaller model on that data. (2) Latency: image encoding adds 100-500ms per request on top of text generation time. Caching helps for repeated images. Dynamic batching of the vision encoder improves throughput since the vision encoder is the bottleneck. (3) Robustness: multimodal models can hallucinate visual details that aren't in the image. I mitigate this with confidence calibration and by cross-referencing VQA answers with OCR output for documents. If the model claims to see text that OCR doesn't detect, flag it.

### Q4: How would you build an image search system using multimodal embeddings?

**Sample Answer:** I'd use CLIP to create aligned image-text embeddings. Index all images by encoding them with CLIP's image encoder and storing the embeddings in a vector database (FAISS or Pinecone). At query time, encode the text query with CLIP's text encoder and find the nearest image embeddings via ANN search. This enables zero-shot image search — no labels needed, and users can search with natural language ("sunset over mountains with snow"). For better results, I'd also index image captions (generated by BLIP-2) and support hybrid search: vector similarity on embeddings plus keyword search on captions. The tradeoff is that CLIP embeddings capture high-level semantics but miss fine-grained details. A query for "red car with license plate ABC-123" will find red cars but won't match specific license plates. For fine-grained attributes, I complement CLIP search with OCR-based filtering.
