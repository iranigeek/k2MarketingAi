package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

	"k2MarketingAi/internal/dataset"
)

func main() {
	var (
		docxPath       = flag.String("docx", "annonser ex.docx", "Path to the .docx file with annonser")
		outputPath     = flag.String("out", "datasets/manual.jsonl", "Where to write the JSONL dataset")
		styleProfileID = flag.String("style-profile-id", "", "Optional style profile ID to store in the dataset")
		styleName      = flag.String("style-profile-name", "", "Optional style profile name metadata")
		tone           = flag.String("tone", "professionell och varm", "Description of the tone for prompts")
		maxBullets     = flag.Int("max-bullets", 8, "Max bullet points extracted from varje annons")
	)
	flag.Parse()

	entries, err := dataset.ExtractParagraphBlocks(*docxPath)
	if err != nil {
		log.Fatalf("parse docx: %v", err)
	}
	if len(entries) == 0 {
		log.Fatalf("no text entries found in %s", *docxPath)
	}

	opts := dataset.RawOptions{
		StyleProfileID:   strings.TrimSpace(*styleProfileID),
		StyleProfileName: strings.TrimSpace(*styleName),
		Tone:             strings.TrimSpace(*tone),
		MaxBullets:       *maxBullets,
	}
	examples, err := dataset.BuildExamplesFromRawEntries(entries, opts)
	if err != nil {
		log.Fatalf("build examples: %v", err)
	}
	if len(examples) == 0 {
		log.Fatal("docx file did not yield any valid training examples")
	}

	if err := dataset.WriteJSONL(*outputPath, examples); err != nil {
		log.Fatalf("write dataset: %v", err)
	}
	log.Printf("exported %d annonser från %s till %s", len(examples), *docxPath, *outputPath)
	fmt.Println("Kör nu ditt finetune-flöde med JSONL-filen och spara modellens namn i stilprofilens custom_model.")
}
