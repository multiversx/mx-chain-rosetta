package provider

import "github.com/multiversx/mx-chain-core-go/data/api"

func removeMiniblocksFromBlock(block *api.Block, shouldRemove func(miniblock *api.MiniBlock) bool) {
	miniblocksToKeep := make([]*api.MiniBlock, 0, len(block.MiniBlocks))

	for _, miniblock := range block.MiniBlocks {
		if shouldRemove(miniblock) {
			continue
		}

		miniblocksToKeep = append(miniblocksToKeep, miniblock)
	}

	block.MiniBlocks = miniblocksToKeep
}

func appendMiniblocksToBlock(block *api.Block, miniblocks []*api.MiniBlock) {
	block.MiniBlocks = append(block.MiniBlocks, miniblocks...)
}
