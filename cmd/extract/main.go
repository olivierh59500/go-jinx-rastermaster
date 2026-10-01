// Command extract recovers the original presentation data.
package main

import (
	"flag"
	"log"
	"os"
	"github.com/olivierh59500/go-jinx-rastermaster/internal/source"
)

func main(){
	input:=flag.String("input","","native executable")
	output:=flag.String("output","","unpacked output executable")
	flag.Parse();if *input==""||*output==""{log.Fatal("-input and -output required")}
	b,err:=os.ReadFile(*input);if err!=nil{log.Fatal(err)};b,err=source.UnpackPRG(b);if err!=nil{log.Fatal(err)};if err=os.WriteFile(*output,b,0644);err!=nil{log.Fatal(err)}
	log.Printf("Recovered %d bytes",len(b))
}
