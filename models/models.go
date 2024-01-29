package models

import (
	"context"
	"fmt"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/rpc"
	confirm "github.com/gagliardetto/solana-go/rpc/sendAndConfirmTransaction"
	"github.com/gagliardetto/solana-go/rpc/ws"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type User struct {
	ID            string    `json:"id" gorm:"primarykey"`
	Email         string    `json:"email" gorm:"unique"`
	EmailVerified bool      `json:"email_verified"`
	PasswordHash  string    `json:"-"`
	Otp           string    `json:"-"`
	OtpExpiresAt  time.Time `json:"-"`
	PrivateKey    string    `json:"-"`
	PublicKey     string    `json:"pubkey"`
}

func (u *User) SendMail(message string) {
	fmt.Println("=====")
	fmt.Println("email to", u.Email)
	fmt.Println(message)
	fmt.Println("=====")
}

func (user *User) ExpireOTP(OTP string, db *gorm.DB) error {
	// TODO: limit to 3 valid calls
	if !user.OtpExpiresAt.Before(time.Now()) && user.Otp == OTP {
		user.OtpExpiresAt = time.Now()
		return db.Save(user).Error
	}
	return fmt.Errorf("Invalid OTP")
}

func (user *User) GetOrCreateSolanaAccount(db *gorm.DB) (*solana.Wallet, error) {
	// TODO: custom iron forge
	client := rpc.New(rpc.DevNet_RPC)

	if user.PrivateKey != "" {
		account, err := solana.WalletFromPrivateKeyBase58(user.PrivateKey)
		client.RequestAirdrop(
			context.TODO(),
			account.PublicKey(),
			solana.LAMPORTS_PER_SOL*1,
			rpc.CommitmentFinalized,
		)
		return account, err
	}

	account := solana.NewWallet()
	out, err := client.RequestAirdrop(
		context.TODO(),
		account.PublicKey(),
		solana.LAMPORTS_PER_SOL*1,
		rpc.CommitmentFinalized,
	)
	fmt.Println("account private key:", account.PrivateKey)
	fmt.Println("account public key:", account.PublicKey())

	// Airdrop 1 SOL to the new account:
	if err != nil {
		return nil, err
	}
	fmt.Println("airdrop transaction signature:", out)

	user.PrivateKey = account.PrivateKey.String()
	user.PublicKey = account.PublicKey().String()

	if err := db.Save(user).Error; err != nil {
		return nil, err
	}

	return account, err
}

func (user *User) MakeTransaction(instructions []solana.Instruction) (*solana.Signature, error) {
	rpcClient := rpc.New(rpc.DevNet_RPC)
	wsClient, err := ws.Connect(context.Background(), rpc.DevNet_WS)
	if err != nil {
		return nil, err
	}
	accountFrom := solana.MustPrivateKeyFromBase58(user.PrivateKey)
	recent, err := rpcClient.GetRecentBlockhash(context.TODO(), rpc.CommitmentFinalized)
	if err != nil {
		return nil, err
	}
	tx, err := solana.NewTransaction(
		instructions,
		recent.Value.Blockhash,
		solana.TransactionPayer(accountFrom.PublicKey()),
	)
	if err != nil {
		return nil, err
	}

	_, err = tx.Sign(
		func(key solana.PublicKey) *solana.PrivateKey {
			if accountFrom.PublicKey().Equals(key) {
				return &accountFrom
			}
			return nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("unable to sign transaction: %w", err)
	}

	sig, err := confirm.SendAndConfirmTransaction(
		context.TODO(),
		rpcClient,
		wsClient,
		tx,
	)
	if err != nil {
		return nil, err
	}

	return &sig, nil
}

func (user *User) MakeTransfer(amount uint64, accountTo solana.PublicKey) (*solana.Signature, error) {
	instructions := []solana.Instruction{
		system.NewTransferInstruction(
			amount,
			solana.MustPublicKeyFromBase58(user.PublicKey),
			accountTo,
		).Build(),
	}
	return user.MakeTransaction(instructions)
}

func ConnectDB() *gorm.DB {
	db, err := gorm.Open(sqlite.Open("develop.db"), &gorm.Config{})
	if err != nil {
		panic("could not connect to db")
	}

	// TODO: handle migrations as a script
	db.AutoMigrate(&User{})

	return db
}
