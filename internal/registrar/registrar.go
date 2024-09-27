package registrar

type Callback func(rowID int64)

type Registrar interface {
	Register(table string, callback Callback, operation int)
}
