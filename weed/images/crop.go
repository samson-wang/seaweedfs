package images

import (
	"bytes"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"

	"github.com/chrislusf/seaweedfs/weed/glog"
	"github.com/oliamb/cutter"
)

func Crop(ext string, data []byte, x, y, width, height int) (cropped []byte, w int, h int) {
	if width == 0 && height == 0 {
		return data, 0, 0
	}
	srcImage, _, err := image.Decode(bytes.NewReader(data))
	if err == nil {
		dstImage, cerr := cutter.Crop(srcImage, cutter.Config{
				Width: width,
				Height: height,
				Anchor: image.Point{x, y},
			})
		if cerr != nil {
			return data, 0, 0
		}
		var buf bytes.Buffer
		switch ext {
		case ".png":
			png.Encode(&buf, dstImage)
		case ".jpg", ".jpeg":
			jpeg.Encode(&buf, dstImage, nil)
		case ".gif":
			gif.Encode(&buf, dstImage, nil)
		}
		return buf.Bytes(), dstImage.Bounds().Dx(), dstImage.Bounds().Dy()
	} else {
		glog.Error(err)
	}
	return data, 0, 0
}
