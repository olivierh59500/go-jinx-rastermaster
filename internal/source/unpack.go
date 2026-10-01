// Package source reads the intro's native presentation data.
package source

import (
	"encoding/binary"
	"fmt"
)

// UnpackPRG reproduces the native backward byte/bitstream decoder. The output
// includes the TOS header and relocation data; it is not relocated in Go.
func UnpackPRG(prg []byte) ([]byte, error) {
	if len(prg)<28 || binary.BigEndian.Uint16(prg)!=0x601a{return nil,fmt.Errorf("source: invalid TOS executable")}
	text:=int(binary.BigEndian.Uint32(prg[2:]));if text<0x308||text>len(prg)-28{return nil,fmt.Errorf("source: incomplete packed program")}
	b:=prg[28:28+text];size:=int(binary.BigEndian.Uint32(b[len(b)-4:]))
	if size<28||size>16<<20{return nil,fmt.Errorf("source: invalid unpacked size")}
	input:=len(b)-4;input-=2;if int16(binary.BigEndian.Uint16(b[input:]))<0{input--};input--
	word:=b[input];var fault error
	read:=func()byte{input--;if input<0x226{fault=fmt.Errorf("source: packed stream underflow");return 0};return b[input]}
	bit:=func()int{carry:=int(word>>7);word<<=1;if word==0{next:=read();word=next<<1|byte(carry);carry=int(next>>7)};return carry}
	bits:=func(n int)int{v:=0;for range n{v=v<<1|bit()};return v}
	out:=make([]byte,size);position:=size
	for position>0{
		length,distance:=0,0
		if bit()==0{
			if bit()==1{length=2;distance=int(read())}else{
				length=bits(3)+1;if length>position{return nil,fmt.Errorf("source: literal exceeds output")};for range length{position--;out[position]=read()};if fault!=nil{return nil,fault};continue
			}
		}else{
			group:=bits(2)
			switch group{
			case 3:length=int(read())+9;if length>position{return nil,fmt.Errorf("source: literal exceeds output")};for range length{position--;out[position]=read()};if fault!=nil{return nil,fault};continue
			case 2:length=int(read())+1;distance=int(read())<<3|bits(3)
			default:length=group+3;distance=int(read())<<(group+1)|bits(group+1)
			}
		}
		if fault!=nil{return nil,fault};if length>position||distance<=0{return nil,fmt.Errorf("source: invalid backward match")}
		for range length{position--;from:=position+distance;if from>=len(out){return nil,fmt.Errorf("source: match exceeds decoded data")};out[position]=out[from]}
	}
	if binary.BigEndian.Uint16(out)!=0x601a{return nil,fmt.Errorf("source: missing decoded TOS header")};return out,nil
}
