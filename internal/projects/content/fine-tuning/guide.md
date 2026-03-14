# Fine-Tuning Open-Source LLMs with LoRA/QLoRA

## Overview

Fine-tuning adapts a pre-trained language model to excel at specific tasks or domains by training on curated datasets. With parameter-efficient methods like LoRA (Low-Rank Adaptation) and QLoRA (Quantized LoRA), you can fine-tune 7B+ parameter models on a single consumer GPU. This makes fine-tuning accessible and practical for production use cases.

This project teaches you the complete fine-tuning workflow: dataset preparation, quantization, adapter training, assessment, and merging. These skills are essential for any role involving LLM customization — from adapting models for specific domains (legal, medical, finance) to creating specialized instruction-following models or aligning outputs with organizational style guides.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    Fine-Tuning Pipeline                         │
│                                                                 │
│  ┌──────────┐   ┌──────────┐   ┌─────────────┐   ┌──────────┐ │
│  │ Dataset   │──▶│ Tokenizer│──▶│ Quantized   │──▶│ LoRA     │ │
│  │ Prep      │   │ + Format │   │ Base Model  │   │ Adapters │ │
│  └──────────┘   └──────────┘   │ (4-bit NF4) │   │ (r=16)   │ │
│                                 └──────┬──────┘   └────┬─────┘ │
│                                        │               │       │
│  ┌──────────┐   ┌──────────┐   ┌──────┴──────┐   ┌────┴─────┐ │
│  │ Deploy    │◀──│ Merged   │◀──│ Assess      │◀──│ SFT      │ │
│  │ (GGUF)   │   │ Model    │   │ (Loss/Qual) │   │ Trainer  │ │
│  └──────────┘   └──────────┘   └─────────────┘   └──────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

**Components:**

- **Dataset Prep** — Converts raw data into instruction/completion pairs. Handles formatting templates (ChatML, Alpaca), train/validation splits, and quality filtering.
- **Tokenizer + Format** — Applies the model's tokenizer with proper chat templates, padding, and truncation. Ensures special tokens are correctly handled.
- **Quantized Base Model** — The pre-trained model loaded in 4-bit NF4 quantization via bitsandbytes, reducing VRAM from ~14GB to ~4GB for a 7B model.
- **LoRA Adapters** — Small trainable matrices (rank 16-64) injected into attention layers. Only 0.1-1% of parameters are trained.
- **SFT Trainer** — Supervised fine-tuning with gradient accumulation, learning rate scheduling, and mixed-precision training.
- **Assess** — Measures training loss convergence, perplexity, and task-specific quality metrics.
- **Merged Model** — LoRA weights merged back into the base model for inference without adapter overhead.

## Key Concepts

### LoRA: Low-Rank Adaptation

LoRA freezes all pre-trained weights and injects trainable low-rank decomposition matrices into each transformer layer. Instead of updating a weight matrix W (d x d), LoRA learns two smaller matrices A (d x r) and B (r x d) where r << d. The effective weight becomes W + BA.

The **rank (r)** controls capacity. r=8 works for simple tasks, r=16-32 for moderate complexity, r=64+ for significant behavior changes. Higher rank means more trainable parameters and more VRAM. The **alpha** parameter scales the LoRA update: `alpha/r` is the effective scaling factor. Setting alpha = 2*r is a common default.

**Target modules** determine which layers get LoRA adapters. For most LLMs, targeting `q_proj`, `v_proj` (attention queries and values) is the minimum. Adding `k_proj`, `o_proj`, `gate_proj`, `up_proj`, `down_proj` trains more parameters but often improves quality for complex tasks.

### QLoRA: Quantized LoRA

QLoRA combines 4-bit quantization of the base model with LoRA training. The base model is quantized to NF4 (Normal Float 4-bit), a data type optimized for normally-distributed neural network weights. Computation happens by dequantizing to BF16 on-the-fly. This reduces VRAM by ~75% with negligible quality loss.

**Double quantization** further compresses the quantization constants themselves, saving an additional ~0.4GB for a 7B model. **Paged optimizers** use CPU RAM to handle VRAM spikes during gradient computation, preventing out-of-memory errors.

### Dataset Formatting

LLMs are sensitive to formatting. Using the wrong template can severely degrade fine-tuning quality. Each model family has its own chat template:

- **ChatML** (used by many models): `<|im_start|>system\n...<|im_end|>\n<|im_start|>user\n...<|im_end|>\n<|im_start|>assistant\n...<|im_end|>`
- **Llama 3**: `<|begin_of_text|><|start_header_id|>system<|end_header_id|>\n\n...<|eot_id|>`

Always use the tokenizer's built-in `apply_chat_template()` method rather than manually formatting strings.

### Learning Rate and Training Duration

Fine-tuning learning rates are much lower than pre-training: 1e-4 to 2e-5 is typical. Too high causes catastrophic forgetting; too low wastes compute. Use a cosine scheduler with warmup (5-10% of steps).

Training for 1-3 epochs is usually sufficient. Watch validation loss — when it starts increasing while training loss continues to decrease, you're overfitting. For small datasets (<1000 examples), even 1 epoch with a low learning rate is appropriate.

## Implementation Steps

### Step 1: Environment Setup

```python
# requirements.txt
torch==2.4.1
transformers==4.44.2
peft==0.12.0
bitsandbytes==0.43.3
trl==0.10.1
datasets==3.0.0
accelerate==0.34.2
```

### Step 2: Dataset Preparation

```python
from datasets import load_dataset, Dataset

def prepare_dataset(data_path: str = None) -> Dataset:
    """Prepare an instruction-tuning dataset.

    Format: each example has 'instruction', 'input' (optional), 'output'.
    """
    # Option 1: Load from HuggingFace Hub
    dataset = load_dataset("tatsu-lab/alpaca", split="train")

    # Option 2: Load from local JSONL
    # dataset = load_dataset("json", data_files=data_path, split="train")

    # Filter low-quality examples
    dataset = dataset.filter(
        lambda x: len(x["output"]) > 20 and len(x["instruction"]) > 10
    )

    # Train/validation split
    split = dataset.train_test_split(test_size=0.05, seed=42)
    return split["train"], split["test"]

def format_for_chat(example, tokenizer):
    """Convert instruction/output to chat format using model's template."""
    messages = []
    if example.get("system"):
        messages.append({"role": "system", "content": example["system"]})

    user_content = example["instruction"]
    if example.get("input"):
        user_content += f"\n\n{example['input']}"
    messages.append({"role": "user", "content": user_content})
    messages.append({"role": "assistant", "content": example["output"]})

    text = tokenizer.apply_chat_template(
        messages, tokenize=False, add_generation_prompt=False
    )
    return {"text": text}
```

### Step 3: Model Loading with QLoRA

```python
import torch
from transformers import (
    AutoModelForCausalLM,
    AutoTokenizer,
    BitsAndBytesConfig,
)
from peft import LoraConfig, get_peft_model, prepare_model_for_kbit_training

def load_model_and_tokenizer(model_name: str = "meta-llama/Llama-3.1-8B-Instruct"):
    # Quantization config for QLoRA
    bnb_config = BitsAndBytesConfig(
        load_in_4bit=True,
        bnb_4bit_quant_type="nf4",
        bnb_4bit_compute_dtype=torch.bfloat16,
        bnb_4bit_use_double_quant=True,
    )

    model = AutoModelForCausalLM.from_pretrained(
        model_name,
        quantization_config=bnb_config,
        device_map="auto",
        attn_implementation="flash_attention_2",  # if supported
        torch_dtype=torch.bfloat16,
    )

    tokenizer = AutoTokenizer.from_pretrained(model_name)
    if tokenizer.pad_token is None:
        tokenizer.pad_token = tokenizer.eos_token
        tokenizer.pad_token_id = tokenizer.eos_token_id

    # Prepare model for training
    model = prepare_model_for_kbit_training(model)

    # LoRA configuration
    lora_config = LoraConfig(
        r=16,
        lora_alpha=32,
        target_modules=[
            "q_proj", "k_proj", "v_proj", "o_proj",
            "gate_proj", "up_proj", "down_proj",
        ],
        lora_dropout=0.05,
        bias="none",
        task_type="CAUSAL_LM",
    )

    model = get_peft_model(model, lora_config)
    model.print_trainable_parameters()
    # Typical output: trainable params: 13,631,488 || all params: 8,043,319,296
    # || trainable%: 0.1695

    return model, tokenizer
```

### Step 4: Training with SFT Trainer

```python
from trl import SFTTrainer, SFTConfig

def train(model, tokenizer, train_dataset, eval_dataset):
    training_args = SFTConfig(
        output_dir="./fine-tuned-model",
        num_train_epochs=3,
        per_device_train_batch_size=4,
        gradient_accumulation_steps=4,  # effective batch size = 16
        learning_rate=2e-4,
        lr_scheduler_type="cosine",
        warmup_ratio=0.05,
        weight_decay=0.01,
        bf16=True,
        logging_steps=10,
        eval_strategy="steps",
        eval_steps=100,
        save_strategy="steps",
        save_steps=100,
        save_total_limit=3,
        load_best_model_at_end=True,
        metric_for_best_model="eval_loss",
        max_seq_length=2048,
        dataset_text_field="text",
        packing=True,  # Pack multiple short examples into one sequence
        report_to="none",  # or "wandb" for experiment tracking
    )

    trainer = SFTTrainer(
        model=model,
        args=training_args,
        train_dataset=train_dataset,
        eval_dataset=eval_dataset,
        tokenizer=tokenizer,
    )

    trainer.train()
    trainer.save_model("./fine-tuned-model/final")
    return trainer
```

### Step 5: Merge and Export

```python
from peft import PeftModel
from transformers import AutoModelForCausalLM, AutoTokenizer
import torch

def merge_and_save(base_model_name: str, adapter_path: str, output_dir: str):
    """Merge LoRA adapters back into the base model for deployment."""
    # Load base model in full precision for merging
    base_model = AutoModelForCausalLM.from_pretrained(
        base_model_name,
        torch_dtype=torch.bfloat16,
        device_map="cpu",
    )
    tokenizer = AutoTokenizer.from_pretrained(base_model_name)

    # Load and merge adapter
    model = PeftModel.from_pretrained(base_model, adapter_path)
    model = model.merge_and_unload()

    model.save_pretrained(output_dir)
    tokenizer.save_pretrained(output_dir)
    print(f"Merged model saved to {output_dir}")
```

## Testing & Evaluation

### Training Metrics

Monitor these during training:
- **Training Loss**: Should decrease smoothly. Spikes indicate data quality issues or learning rate too high.
- **Validation Loss**: Should track training loss. Divergence indicates overfitting.
- **Gradient Norm**: Should be stable (1-10 range). Spikes indicate instability.

### Quality Assessment

```python
def assess_generation(model, tokenizer, test_prompts: list[dict]) -> list:
    """Generate responses for test prompts and collect for human review."""
    results = []
    model.eval()
    for prompt in test_prompts:
        messages = [{"role": "user", "content": prompt["instruction"]}]
        input_ids = tokenizer.apply_chat_template(
            messages, return_tensors="pt", add_generation_prompt=True
        ).to(model.device)

        with torch.no_grad():
            output = model.generate(
                input_ids,
                max_new_tokens=512,
                temperature=0.7,
                top_p=0.9,
                do_sample=True,
            )
        response = tokenizer.decode(
            output[0][input_ids.shape[1]:], skip_special_tokens=True
        )
        results.append({
            "instruction": prompt["instruction"],
            "expected": prompt.get("expected_output", ""),
            "generated": response,
        })
    return results
```

### Automated Metrics

- **Perplexity** on held-out data: lower is better, but compare against the base model as a baseline.
- **Task-specific accuracy**: if fine-tuning for classification or extraction, measure exact match or F1.
- **ROUGE/BERTScore**: for summarization or generation tasks, compare generated text against references.

## Interview Angles

### Q1: What's the difference between LoRA and full fine-tuning, and when would you choose each?

**Sample Answer:** Full fine-tuning updates all model parameters, which gives maximum flexibility to learn new behaviors but requires substantial VRAM (2x model size for optimizer states + gradients) and risks catastrophic forgetting. LoRA freezes all original weights and adds small trainable matrices (typically 0.1-1% of total parameters) to attention layers. LoRA is preferable for most production use cases because it needs 10-50x less VRAM, trains faster, produces small adapter files (50-200MB vs 14GB+), and supports serving multiple adapters from one base model. Full fine-tuning is justified when you need fundamental behavior changes, have very large datasets (100k+ examples), or are training from a base model (not instruct-tuned). In practice, LoRA with r=32 on all linear layers achieves 95-98% of full fine-tuning quality on most benchmarks.

### Q2: How do you prevent catastrophic forgetting during fine-tuning?

**Sample Answer:** Catastrophic forgetting happens when fine-tuning overwrites general capabilities while learning task-specific ones. I use several strategies: (1) Low learning rate with cosine schedule — 1e-4 to 2e-5 prevents aggressive weight updates. (2) LoRA inherently mitigates forgetting because original weights are frozen. (3) Mix in general-purpose data — blend 10-20% of a general instruction dataset with task-specific data to maintain broad capabilities. (4) Short training duration — 1-3 epochs is usually sufficient; extended training on narrow data accelerates forgetting. (5) Validation on both task-specific and general benchmarks during training to detect degradation early. The tradeoff is that stronger forgetting prevention generally means slower adaptation to the target task.

### Q3: How do you decide on the LoRA rank (r) and which layers to target?

**Sample Answer:** I start with r=16 targeting all attention projections (q, k, v, o) and MLP layers (gate, up, down). This covers ~0.2% of parameters for a 7B model. If the task is simple (style adaptation, format compliance), r=8 on just q_proj and v_proj is sufficient. For complex tasks requiring new knowledge or reasoning patterns, I increase to r=32 or r=64. I select rank empirically: train with r=16, check results, then try r=8 (to see if quality holds) and r=32 (to see if quality improves). There are diminishing returns beyond r=64. For alpha, I use alpha = 2*r as default. The total trainable parameter count should be proportional to your dataset size — if you have only 1000 examples, a high rank will overfit.

### Q4: What data quality issues most commonly cause fine-tuning to fail?

**Sample Answer:** The top issues I've seen: (1) Inconsistent formatting — mixing templates or having mismatched special tokens causes the model to learn noise. Always use the tokenizer's apply_chat_template. (2) Label errors — even 5% incorrect outputs can teach wrong behaviors. I manually audit a random 50-example sample before training. (3) Imbalanced categories — if 90% of examples are one type, the model learns to always produce that type. Upsample minority classes or use stratified batching. (4) Examples that exceed max sequence length silently get truncated, losing the output portion. Filter or split long examples. (5) Duplicate or near-duplicate examples cause the model to memorize rather than generalize. Deduplicate using embedding similarity with a threshold of 0.95.

### Q5: How would you serve multiple LoRA fine-tuned models efficiently?

**Sample Answer:** Since LoRA adapters are small (50-200MB), you can serve many from one base model. The approach is: load the base model once in GPU memory, then dynamically load/unload LoRA adapters per request. Libraries like vLLM and LoRAX support this natively — they batch requests across different adapters, sharing the base model computation and only diverging at the adapter layers. This is dramatically more efficient than deploying separate full models. The tradeoff is added routing complexity and slightly higher per-request latency (adapter switching). For the highest throughput, group requests by adapter and process in batches. If adapters are rarely switched, keep the most popular ones warm and load others on-demand with a small latency penalty (50-100ms for adapter loading).
