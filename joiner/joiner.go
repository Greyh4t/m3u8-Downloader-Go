package joiner

type Joiner interface {
	Add(id int, block []byte) error
	Merge() error
}
