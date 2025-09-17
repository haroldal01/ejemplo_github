package OutPut

import (
	"bytes"
	"fmt"
)

var outputBuffer bytes.Buffer

func Println(a ...any) {
	fmt.Println(a...) // Imprime en consola normal
	fmt.Fprintln(&outputBuffer, a...)
}

func Printf(format string, a ...any) {
	fmt.Printf(format, a...) // Imprime en consola normal
	fmt.Fprintf(&outputBuffer, format, a...)
}

func GetOutput() string {
	return outputBuffer.String()
}

func Clear() {
	outputBuffer.Reset()
}
