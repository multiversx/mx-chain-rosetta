package testscommon

var (
	// TODO: use "testAccount", instead
	// TestAddressAlice is a test address
	TestAddressAlice = "erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th"
	// TestPubKeyAlice is a test pubkey
	TestPubKeyAlice, _ = RealWorldBech32PubkeyConverter.Decode(TestAddressAlice)

	// TODO: use "testAccount", instead
	// TestAddressBob is a test address
	TestAddressBob = "erd1spyavw0956vq68xj8y4tenjpq2wd5a9p2c6j8gsz7ztyrnpxrruqzu66jx"
	// TestPubKeyBob is a test pubkey
	TestPubKeyBob, _ = RealWorldBech32PubkeyConverter.Decode(TestAddressBob)

	// TODO: use "testAccount", instead
	// TestAddressCarol is a test address
	TestAddressCarol = "erd1k2s324ww2g0yj38qn2ch2jwctdy8mnfxep94q9arncc6xecg3xaq6mjse8"
	// TestPubKeyCarol is a test pubkey
	TestPubKeyCarol, _ = RealWorldBech32PubkeyConverter.Decode(TestAddressCarol)

	// TODO: use "testAccount", instead
	// TestAddressOfContract is a test address
	TestAddressOfContract = "erd1qqqqqqqqqqqqqpgqfejaxfh4ktp8mh8s77pl90dq0uzvh2vk396qlcwepw"

	// TestUserAShard0 is a test account (user)
	TestUserAShard0 = newTestAccount("erd1spyavw0956vq68xj8y4tenjpq2wd5a9p2c6j8gsz7ztyrnpxrruqzu66jx")

	// TestUserBShard0 is a test account (user)
	TestUserBShard0 = newTestAccount("erd1uv40ahysflse896x4ktnh6ecx43u7cmy9wnxnvcyp7deg299a4sq6vaywa")

	// TestUserCShard0 is a test account (user)
	TestUserCShard0 = newTestAccount("erd1ncsyvhku3q7zy8f8rjmmx2t9zxgch38cel28kzg3m8pt86dt0vqqecw0gy")

	// TestContractFooShard0 is a test account (contract)
	TestContractFooShard0 = newTestAccount("erd1qqqqqqqqqqqqqpgqagjekf5mxv86hy5c62vvtug5vc6jmgcsq6uq8reras")

	// TestContractBarShard0 is a test account (contract)
	TestContractBarShard0 = newTestAccount("erd1qqqqqqqqqqqqqpgqdstpe4fepzl4w8683xw88t5kcjkxz0zaq6uquj6ztu")

	// TestContractFooShard1 is a test account (contract)
	TestContractFooShard1 = newTestAccount("erd1qqqqqqqqqqqqqpgq89t5xm4x04tnt9lv747wdrsaycf3rcwcggzsa7crse")

	// TestContractBarShard1 is a test account (contract)
	TestContractBarShard1 = newTestAccount("erd1qqqqqqqqqqqqqpgq0dtujxcrmwwqdtwzvq5nxuwgjcgaty7fggzse2vmm2")

	// TestContractFooShard2 is a test account (contract)
	TestContractFooShard2 = newTestAccount("erd1qqqqqqqqqqqqqpgqeesfamasje5zru7ku79m8p4xqfqnywvqxj0qhtyzdr")

	// TestContractBarShard2 is a test account (contract)
	TestContractBarShard2 = newTestAccount("erd1ux2wvqyh8pw8ea26urjqq65mqytfn42dr980pvucztxk9w79xj0q2x98te")

	// TestUserShard1 is a test account (user)
	TestUserShard1 = newTestAccount("erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th")

	// TestUserShard2 is a test account (user)
	TestUserShard2 = newTestAccount("erd1k2s324ww2g0yj38qn2ch2jwctdy8mnfxep94q9arncc6xecg3xaq6mjse8")
)

type testAccount struct {
	Address string
	PubKey  []byte
}

func newTestAccount(address string) *testAccount {
	pubKey, _ := RealWorldBech32PubkeyConverter.Decode(address)

	return &testAccount{
		Address: address,
		PubKey:  pubKey,
	}
}
