package baconsigner

import (
	"bakinbacon/storage"
	"github.com/pkg/errors"
)

// SignerAccess is an abstraction over accessing either supported signer
type SignerAccess interface {
	CreateWalletSigner() *WalletSigner
	GetWalletSigner() (*WalletSigner, error)
	CreateLedgerSigner() *LedgerSigner
	GetLedgerSigner() (*LedgerSigner, error)
	ConfirmBakingPkh(pkh, bip string) error
}

type Access struct {
	walletSigner *WalletSigner
	ledgerSigner *LedgerSigner
	storage      *storage.Storage
}

func (s *Access) CreateWalletSigner() *WalletSigner {
	return &WalletSigner{storage: s.storage}
}


func (s *Access) GetWalletSigner() (*WalletSigner, error) {
	if s.walletSigner != nil {
		return s.walletSigner, nil
	}

	walletSigner, err := s.HydrateWalletSigner()
	if err != nil {
		return nil, errors.Wrap(err, "Cannot init wallet signer")
	}

	return walletSigner, nil
}

func (s *Access) CreateLedgerSigner() *LedgerSigner {
	return &LedgerSigner{Storage: s.storage}
}

func (s *Access) GetLedgerSigner() (*LedgerSigner, error) {

	if s.ledgerSigner != nil {
		return s.ledgerSigner, nil
	}

	ledgerSigner, err := s.HydrateLedgerSigner()
	if err != nil {
		return nil, errors.Wrap(err, "Cannot init ledger signer")
	}

	return ledgerSigner, nil
}

func (s *Access) ConfirmBakingPkh(pkh, bip string) error {

	if s.ledgerSigner == nil {
		return errors.New("ledger signer is not instantiated")
	}

	return s.ledgerSigner.ConfirmBakingPkh(pkh, bip)
}

func NewSignerAccess(db *storage.Storage) SignerAccess {
	return &Access{storage: db}
}
