package questmultipart

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"mime/multipart"
)

type Form struct {
	Buffer *bytes.Buffer
	Writer *multipart.Writer
	Err    error
}

type Encoder interface {
	Encode(io.Writer, interface{}) error
}

func New() *Form {
	buffer := &bytes.Buffer{}
	writer := multipart.NewWriter(buffer)
	return &Form{buffer, writer, nil}
}

func (f *Form) AddFile(fieldName string, fileName string, value interface{}, encoder Encoder) *Form {
	fileWriter, err := f.Writer.CreateFormFile(fieldName, fileName)
	if err != nil {
		f.Err = err
		return f
	}
	err = encoder.Encode(fileWriter, value)
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

type XMLEncoder struct{}

func (x *XMLEncoder) Encode(w io.Writer, data interface{}) error {
	fmt.Fprintf(w, "%s\n", xml.Header)
	enc := xml.NewEncoder(w)
	return enc.Encode(data)
}

type JSONEncoder struct{}

func (x *JSONEncoder) Encode(w io.Writer, data interface{}) error {
	enc := json.NewEncoder(w)
	return enc.Encode(data)
}

type CopyEncoder struct{}

func (x *CopyEncoder) Encode(w io.Writer, data interface{}) error {
	_, err := io.Copy(w, data.(io.Reader))
	return err
}
