// error.go
package base

type DTerror struct {
	Reason string
}

func (e DTerror) Error() string {
	return e.Reason
}
