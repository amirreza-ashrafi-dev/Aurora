package presentation

import (
	_ "embed"
	"sync"

	"github.com/egoist/mygo/ui"
)

//go:embed assets/logo.png
var auroraLogoPNG []byte

var (
	auroraLogoOnce sync.Once
	auroraLogoBitmap *ui.Bitmap
)

func auroraLogo() *ui.Bitmap {
	auroraLogoOnce.Do(func() {
		bitmap, err := ui.DecodeBitmap(auroraLogoPNG)
		if err == nil {
			auroraLogoBitmap = bitmap
		}
	})
	return auroraLogoBitmap
}
