package asset_test

import (
	"github.com/pixiv/go-libjpeg/jpeg"
	nativeJpeg "image/jpeg"
	"io/ioutil"
	"os"
	"testing"
)

func BenchmarkEncodeDecodeLibJpeg(b *testing.B) {
	image, openErr := os.Open("../../../testdata/spec.jpg")
	if openErr != nil {
		b.Fatal(openErr)
	}
	defer image.Close()
	img, decodeErr := jpeg.Decode(image, &jpeg.DecoderOptions{})
	if decodeErr != nil {
		b.Fatal(decodeErr)
	}
	tempFile, tempErr := ioutil.TempFile("", "*")
	if tempErr != nil {
		b.Fatal(tempErr)
	}
	defer os.Remove(tempFile.Name())
	if err := jpeg.Encode(tempFile, img, &jpeg.EncoderOptions{Quality: 85}); err != nil {
		b.Fatal(err)
	}
}

func BenchmarkEncodeDecodeNativeJpeg(b *testing.B) {
	image, openErr := os.Open("../../../testdata/spec.jpg")
	if openErr != nil {
		b.Fatal(openErr)
	}
	defer image.Close()
	img, decodeErr := nativeJpeg.Decode(image)
	if decodeErr != nil {
		b.Fatal(decodeErr)
	}
	tempFile, tempErr := ioutil.TempFile("", "*")
	if tempErr != nil {
		b.Fatal(tempErr)
	}
	defer os.Remove(tempFile.Name())
	if err := nativeJpeg.Encode(tempFile, img, &nativeJpeg.Options{Quality: 85}); err != nil {
		b.Fatal(err)
	}
}
