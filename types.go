package main

type Nameable interface {
	GetName() string
}

type BuiltinAlias struct {
	Name     string
	Desc     string
	Template string
	Handler  CommandFunc
}

func (b BuiltinAlias) GetName() string {
	return b.Name
}
