package dataset

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// ExtractParagraphBlocks reads a docx file and returns paragraph groups separated by blank lines.
func ExtractParagraphBlocks(path string) ([]string, error) {
	paragraphs, err := readDocxParagraphs(path)
	if err != nil {
		return nil, err
	}
	var entries []string
	var current []string
	for _, paragraph := range paragraphs {
		if strings.TrimSpace(paragraph) == "" {
			if len(current) > 0 {
				entries = append(entries, strings.Join(current, "\n\n"))
				current = nil
			}
			continue
		}
		current = append(current, strings.TrimSpace(paragraph))
	}
	if len(current) > 0 {
		entries = append(entries, strings.Join(current, "\n\n"))
	}
	return entries, nil
}

func readDocxParagraphs(path string) ([]string, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("open docx: %w", err)
	}
	defer reader.Close()

	var documentXML []byte
	for _, file := range reader.File {
		if file.Name == "word/document.xml" {
			rc, err := file.Open()
			if err != nil {
				return nil, fmt.Errorf("open document.xml: %w", err)
			}
			data, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return nil, fmt.Errorf("read document.xml: %w", err)
			}
			documentXML = data
			break
		}
	}
	if len(documentXML) == 0 {
		return nil, fmt.Errorf("no word/document.xml found")
	}

	decoder := xml.NewDecoder(bytes.NewReader(documentXML))
	var paragraphs []string
	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("parse document.xml: %w", err)
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "p" {
			continue
		}
		text, err := collectParagraph(decoder)
		if err != nil {
			return nil, err
		}
		paragraphs = append(paragraphs, text)
	}
	return paragraphs, nil
}

func collectParagraph(decoder *xml.Decoder) (string, error) {
	var builder strings.Builder
	depth := 1
	for depth > 0 {
		token, err := decoder.Token()
		if err != nil {
			return "", err
		}
		switch tok := token.(type) {
		case xml.StartElement:
			depth++
			switch tok.Name.Local {
			case "t":
				var text string
				if err := decoder.DecodeElement(&text, &tok); err != nil {
					return "", err
				}
				builder.WriteString(text)
				depth--
			case "tab":
				builder.WriteRune('\t')
			case "br", "cr":
				builder.WriteRune('\n')
			}
		case xml.CharData:
			builder.Write(tok)
		case xml.EndElement:
			depth--
		}
	}
	return builder.String(), nil
}
