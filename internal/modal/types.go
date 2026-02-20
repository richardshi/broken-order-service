package modal

type IssueType string

const (
	IssueTransferFailed IssueType = "TRANSFER_FAILED"
	IssuePaymentFailed IssueType = "PAYMENT_FAILED"
)

type TransferStatus string

const (
	TransferNotAccepted TransferStatus = "NOT_ACCEPTED"
	TransferAccepted TransferStatus = "ACCEPTED"
)

