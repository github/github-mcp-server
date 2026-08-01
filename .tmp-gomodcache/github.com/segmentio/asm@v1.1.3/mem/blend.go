package mem

func blendGeneric(dst, src []byte) int {
	switch {
	case len(dst) < len(src):
		src = src[:len(dst)]
	case len(dst) > len(src):
		dst = dst[:len(src)]
	}

	for i := range dst {
		dst[i] |= src[i]
	}

	return len(dst)
}
