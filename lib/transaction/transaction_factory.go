package transaction

func NewTransactionMgr() ITransactionMgr {
	return new(TransactionMgr)
}