package provider

import "github.com/ElrondNetwork/elrond-proxy-go/data"

func removeMiniblocksFromBlock(block *data.Block, shouldRemove func(miniblock *data.MiniBlock) bool) {
	miniblocksToKeep := make([]*data.MiniBlock, 0, len(block.MiniBlocks))

	for _, miniblock := range block.MiniBlocks {
		if shouldRemove(miniblock) {
			continue
		}

		miniblocksToKeep = append(miniblocksToKeep, miniblock)
	}

	block.MiniBlocks = miniblocksToKeep
}

func appendMiniblocksToBlock(block *data.Block, miniblocks []*data.MiniBlock) {
	block.MiniBlocks = append(block.MiniBlocks, miniblocks...)
}
