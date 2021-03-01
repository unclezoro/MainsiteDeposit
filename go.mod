module MainsiteDeposit

require (
	github.com/ethereum/go-ethereum v1.9.12
	github.com/stretchr/testify v1.4.0
	github.com/binance-chain/go-sdk v1.2.6
)

replace github.com/tendermint/go-amino => github.com/binance-chain/bnc-go-amino v0.14.1-binance.1

go 1.15
