package brand

import _ "embed"

//go:embed icon.png
var PNG []byte

//go:embed icon-ui.png
var SmallPNG []byte

//go:embed remote-rabbit.png
var RemoteRabbitPNG []byte

//go:embed up-to-date-rabbit.png
var UpToDateRabbitPNG []byte

//go:embed empty-diff.png
var EmptyDiffPNG []byte

//go:embed empty-preview.png
var EmptyPreviewPNG []byte
