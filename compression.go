package libstandard

import (
	"bytes"

	"github.com/andybalholm/brotli"
)

func Compress(data []byte) ([]byte, error) {
	srcBuf := bytes.NewBuffer(data)
	dstBuf := bytes.NewBuffer(make([]byte, 0))
	writer := brotli.NewWriterLevel(dstBuf, 11)
	_, err := srcBuf.WriteTo(writer)
	if err != nil {
		return nil, err
	}

	err = writer.Close()
	if err != nil {
		return nil, err
	}

	return dstBuf.Bytes(), nil
}

func Decompress(data []byte) ([]byte, error) {
	srcBuf := bytes.NewBuffer(data)
	dstBuf := bytes.NewBuffer(make([]byte, 0))
	reader := brotli.NewReader(srcBuf)
	_, err := dstBuf.ReadFrom(reader)
	return dstBuf.Bytes(), err
}
