package balanceLimit

import (
	"context"
	"crypto/ecdsa"
	"math"
	"math/big"
	"math/rand"
	"testing"
	"time"

	"github.com/klaytn/klaytn/accounts/abi/bind"
	"github.com/klaytn/klaytn/accounts/abi/bind/backends"
	"github.com/klaytn/klaytn/blockchain"
	"github.com/klaytn/klaytn/blockchain/types"
	"github.com/klaytn/klaytn/blockchain/types/account"
	"github.com/klaytn/klaytn/crypto"
	"github.com/klaytn/klaytn/params"
	"github.com/stretchr/testify/assert"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

type balanceLimitEOATestEnv struct {
	backend             *backends.SimulatedBackend
	sender              []*bind.TransactOpts
	receiver            []*bind.TransactOpts
	initialBalanceLimit *big.Int
}

func generateBalanceLimitEOATestEnv(t *testing.T) *balanceLimitEOATestEnv {
	senderNum := 1
	receiverNum := 3
	accountNum := senderNum + receiverNum
	keys := make([]*ecdsa.PrivateKey, accountNum)
	accounts := make([]*bind.TransactOpts, accountNum)
	for i := 0; i < accountNum; i++ {
		keys[i], _ = crypto.GenerateKey()
		accounts[i] = bind.NewKeyedTransactor(keys[i])
		accounts[i].GasLimit = DefaultGasLimit
	}

	// generate backend with deployed
	alloc := blockchain.GenesisAlloc{}
	for i := 0; i < senderNum; i++ {
		alloc[accounts[i].From] = blockchain.GenesisAccount{
			Balance: big.NewInt(params.KLAY),
		}
	}
	backend := backends.NewSimulatedBackend(alloc)

	return &balanceLimitEOATestEnv{
		backend:             backend,
		sender:              accounts[0:senderNum],
		receiver:            accounts[senderNum:],
		initialBalanceLimit: account.GetInitialBalanceLimit(),
	}
}

func setBalanceLimit(backend *backends.SimulatedBackend, from *bind.TransactOpts, balanceLimit *big.Int, t *testing.T) *types.Transaction {
	ctx := context.Background()

	nonce, err := backend.NonceAt(ctx, from.From, nil)
	assert.NoError(t, err)

	chainID, err := backend.ChainID(ctx)
	assert.NoError(t, err)

	tx, err := types.NewTransactionWithMap(types.TxTypeBalanceLimitUpdate, map[types.TxValueKeyType]interface{}{
		types.TxValueKeyNonce:        nonce,
		types.TxValueKeyGasLimit:     0,
		types.TxValueKeyGasPrice:     big.NewInt(0),
		types.TxValueKeyFrom:         from.From,
		types.TxValueKeyBalanceLimit: balanceLimit,
	})
	assert.NoError(t, err)

	signedTx, err := from.Signer(types.NewEIP155Signer(chainID), from.From, tx)
	assert.NoError(t, err)
	err = backend.SendTransaction(context.Background(), signedTx)
	assert.NoError(t, err)

	return signedTx
}

// 존재하지 않는 account에 대해 getBalanceLimit을 호출할 때, ErrNilAccount(Account not set) 에러 발생
func TestBalanceLimit_EOA_getBalanceLimit_newAccount(t *testing.T) {
	env := generateBalanceLimitEOATestEnv(t)
	defer env.backend.Close()

	backend := env.backend
	receiver := env.receiver[0]

	// getBalanceLimit 호출 시 실패
	limit, err := backend.BalanceLimitAt(context.Background(), receiver.From, nil)
	assert.Equal(t, account.ErrNilAccount, err)
	assert.Equal(t, big.NewInt(0), limit)
}

// 존재하지 않는 account에 대해 setBalanceLimit을 호출할 때, BalanceLimit 설정하는 것을 확인
func TestBalanceLimit_EOA_setBalanceLimit_newAccount(t *testing.T) {
	env := generateBalanceLimitEOATestEnv(t)
	defer env.backend.Close()

	backend := env.backend
	sender := env.sender[0]
	balanceLimit := big.NewInt(100)

	// setBalanceLimit 호출
	tx := setBalanceLimit(backend, sender, balanceLimit, t)
	backend.Commit()
	CheckReceipt(backend, tx, 1*time.Second, types.ReceiptStatusSuccessful, t)

	//klay_BalanceLimit 실행 시 설정한 값이 셋팅된 것을 확인
	limit, err := backend.BalanceLimitAt(context.Background(), sender.From, nil)
	assert.NoError(t, err)
	assert.Equal(t, balanceLimit, limit)
}

// 존재하는 account에 대해 getBalanceLimit을 호출할 때, 초기값 출력 InitialBalanceLimit 확인
func TestBalanceLimit_EOA_InitialBalanceLimit(t *testing.T) {
	env := generateBalanceLimitEOATestEnv(t)
	defer env.backend.Close()

	backend := env.backend
	sender := env.sender[0]
	receiver := env.receiver[0]

	// A → B로 value transfer (B를 EOA로 설정)
	min := 0
	max := math.MaxInt32
	transferAmount := big.NewInt(int64(rand.Intn(max-min) + min))
	tx, err := ValueTransfer(backend, sender, receiver.From, transferAmount, t)
	assert.NoError(t, err)
	backend.Commit()
	CheckReceipt(backend, tx, 1*time.Second, types.ReceiptStatusSuccessful, t)

	// B에 대해 getBalanceLimit 실행하여 InitialBalanceLimit 조회
	limit, err := backend.BalanceLimitAt(context.Background(), sender.From, nil)
	assert.NoError(t, err)
	assert.Equal(t, account.GetInitialBalanceLimit(), limit)
}