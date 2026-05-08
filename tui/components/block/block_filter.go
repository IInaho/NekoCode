// block_filter.go — FilterFinalBlocks：过滤掉 ephemeral 块，仅保留工具调用等持久块。
package block

func FilterFinalBlocks(blocks []ContentBlock) []ContentBlock {
	out := make([]ContentBlock, 0, len(blocks))
	for _, b := range blocks {
		switch b.Type {
		case BlockTool:
			out = append(out, b)
		}
	}
	return out
}
