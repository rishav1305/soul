package generate

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// float64Ptr returns a pointer to a float64 value.
func float64Ptr(f float64) *float64 { return &f }

// HTMLToPDF renders the given HTML string to a PDF file at outputPath
// using a headless Chrome instance via Rod.
func HTMLToPDF(html string, outputPath string) error {
	// Ensure the output directory exists.
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create output dir %s: %w", dir, err)
	}

	// Launch headless Chrome.
	u, err := launcher.New().
		Headless(true).
		Launch()
	if err != nil {
		return fmt.Errorf("launch headless chrome: %w", err)
	}

	browser := rod.New().ControlURL(u)
	if err := browser.Connect(); err != nil {
		return fmt.Errorf("connect to chrome: %w", err)
	}
	defer browser.MustClose()

	// Create a new page.
	page, err := browser.Page(proto.TargetCreateTarget{})
	if err != nil {
		return fmt.Errorf("create page: %w", err)
	}

	// Set the HTML content.
	if err := page.SetDocumentContent(html); err != nil {
		return fmt.Errorf("set document content: %w", err)
	}

	// Wait for rendering to complete.
	time.Sleep(1 * time.Second)

	// Print to PDF with A4 dimensions.
	reader, err := page.PDF(&proto.PagePrintToPDF{
		PrintBackground: true,
		PaperWidth:      float64Ptr(8.27),  // A4 width in inches
		PaperHeight:     float64Ptr(11.69), // A4 height in inches
		MarginTop:       float64Ptr(0.4),
		MarginBottom:    float64Ptr(0.4),
		MarginLeft:      float64Ptr(0.5),
		MarginRight:     float64Ptr(0.5),
	})
	if err != nil {
		return fmt.Errorf("print to PDF: %w", err)
	}
	defer reader.Close()

	// Read the PDF bytes from the stream.
	pdfBytes, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("read PDF stream: %w", err)
	}

	// Write to file.
	if err := os.WriteFile(outputPath, pdfBytes, 0o644); err != nil {
		return fmt.Errorf("write PDF to %s: %w", outputPath, err)
	}

	return nil
}
