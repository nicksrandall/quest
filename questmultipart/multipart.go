package questmultipart

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"mime/multipart"

	jsoniter "github.com/json-iterator/go"
)

type Form struct {
	Buffer *bytes.Buffer
	Writer *multipart.Writer
	Err    error
}

type Encoder func(io.Writer, interface{}) error

func New() *Form {
	buffer := &bytes.Buffer{}
	writer := multipart.NewWriter(buffer)
	return &Form{buffer, writer, nil}
}

func (f *Form) AddFile(fieldName, fileName string, value interface{}, encoder Encoder) *Form {
	fileWriter, err := f.Writer.CreateFormFile(fieldName, fileName)
	if err != nil {
		f.Err = err
		return f
	}
	err = encoder(fileWriter, value)
	if err != nil {
		f.Err = err
		return f
	}
	return f
}

func (f *Form) AddField(name, value string) *Form {
	err := f.Writer.WriteField(name, value)
	if err != nil {
		f.Err = err
		return f
	}
	return f
}

func (f *Form) Close() *Form {
	err := f.Writer.Close()
	if err != nil {
		f.Err = err
		return f
	}
	return f
}

func XMLEncode(w io.Writer, data interface{}) error {
	fmt.Fprintf(w, "%s\n", xml.Header)
	enc := xml.NewEncoder(w)
	return enc.Encode(data)
}

func JSONEncode(w io.Writer, data interface{}) error {
	enc := jsoniter.NewEncoder(w)
	return enc.Encode(data)
}

func CopyEncode(w io.Writer, data interface{}) error {
	_, err := io.Copy(w, data.(io.Reader))
	return err
}
