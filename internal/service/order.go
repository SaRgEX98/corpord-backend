package service

type order struct {
}

type Order interface {
	All()
}

func NewOrder() Order {
	return &order{}
}

func (o *order) All() {
	//TODO implement me
	panic("implement me")
}
