package internal

func Ref[k any](input k) *k {
	return &input
}
