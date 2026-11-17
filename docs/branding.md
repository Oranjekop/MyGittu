# MyGittu 应用图标

图标使用内置 imagegen 工具生成，主文件为 `internal/brand/source.png`。采用 Q 版奶白兔子、弯耳、薄荷绿底色和橙色 Git 胸针。生成结果已人工检查，没有使用外部角色或商标参考。

## 生成提示词

Create ONE beautiful finished app icon for MyGittu, a Git desktop client, featuring a very cute original chibi bunny mascot. Big rounded cream-white bunny head, two soft short upright ears (one slightly bent), tiny dark cocoa dot eyes, a tiny smiling mouth, subtle peach blush cheeks, small rounded paws peeking at the bottom. Friendly confident expression. Centered bunny portrait on a muted sage / soft mint rounded-square tile. A tiny tasteful orange Git branch pin on one side of its chest, subordinate to the bunny and readable as a simple three-node fork. Premium contemporary Japanese kawaii mascot design, clean soft 2.5D clay illustration with very restrained matte shading, generous simple shapes, crisp charming silhouette at small size, visually balanced. Bunny fills most of the tile with ears fully within bounds, no cropping. No shiny plastic, glass, metallic, fluorescent green, excessive bevels, scenery, text, letters, watermark, frame, collage, multiple variants. Transparent outside the rounded square tile, clean sharp alpha, icon artwork only, square canvas, high resolution.

## 输出资源

`scripts/build-icons.cjs` 使用 sharp 将主文件按比例转换为 `icon.png`、`icon-ui.png`、Windows ICO 和 macOS ICNS，保留透明通道。只做格式与尺寸转换，不重绘角色。

修改主文件后，运行 `node scripts/build-icons.cjs`（需要 sharp），再运行 `go generate ./cmd/desktop` 更新 Windows x64/ARM64 资源。小尺寸 Bitmap 只在启动时解码一次。
