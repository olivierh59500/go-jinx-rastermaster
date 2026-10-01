// Command extract recovers the original presentation data.
package main

import (
	"flag"
	"github.com/olivierh59500/go-jinx-rastermaster/internal/source"
	"log"
	"os"
)

func main() {
	input := flag.String("input", "", "native executable")
	output := flag.String("output", "", "unpacked output executable")
	directory := flag.String("assets", "", "decoded presentation directory")
	flag.Parse()
	if *input == "" || (*output == "" && *directory == "") {
		log.Fatal("-input and either -output or -assets required")
	}
	b, err := os.ReadFile(*input)
	if err != nil {
		log.Fatal(err)
	}
	b, err = source.UnpackPRG(b)
	if err != nil {
		log.Fatal(err)
	}
	if *output != "" {
		if err = os.WriteFile(*output, b, 0644); err != nil {
			log.Fatal(err)
		}
	}
	if *directory != "" {
		if err = source.ExportAssets(b, *directory); err != nil {
			log.Fatal(err)
		}
	}
	log.Printf("Recovered %d bytes", len(b))
}
