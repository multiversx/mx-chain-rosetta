package provider

import (
	"testing"

	"github.com/multiversx/mx-chain-core-go/data/api"
	"github.com/stretchr/testify/require"
)

func TestRemoveMiniblocksFromBlock(t *testing.T) {
	block := &api.Block{
		MiniBlocks: []*api.MiniBlock{
			{Hash: "aaaa", SourceShard: 7},
			{Hash: "bbbb", SourceShard: 8},
			{Hash: "aabb", SourceShard: 7},
			{Hash: "abba", SourceShard: 8},
		},
	}

	shouldRemove := func(miniblock *api.MiniBlock) bool {
		return miniblock.SourceShard == 8
	}

	removeMiniblocksFromBlock(block, shouldRemove)
	require.Len(t, block.MiniBlocks, 2)
	require.Equal(t, "aaaa", block.MiniBlocks[0].Hash)
	require.Equal(t, "aabb", block.MiniBlocks[1].Hash)
}

func TestAppendMiniblocksToBlock(t *testing.T) {
	block := &api.Block{
		MiniBlocks: []*api.MiniBlock{
			{Hash: "aaaa"},
			{Hash: "bbbb"},
		},
	}

	appendMiniblocksToBlock(block, []*api.MiniBlock{{Hash: "abcd"}, {Hash: "dcba"}})
	require.Len(t, block.MiniBlocks, 4)
	require.Equal(t, "abcd", block.MiniBlocks[2].Hash)
	require.Equal(t, "dcba", block.MiniBlocks[3].Hash)
}
