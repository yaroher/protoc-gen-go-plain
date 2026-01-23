package cast

// IntoPtr возвращает указатель на переданное значение
func IntoPtr[T any](v T) *T {
	return &v
}

type Caster[A any, B any] interface {
	Cast(v A) B
}

type CasterErr[A any, B any] interface {
	CastErr(v A) (B, error)
}

type CasterFunc[A any, B any] func(A) B

func (c CasterFunc[A, B]) Cast(v A) B {
	return c(v)
}

func CasterFn[A any, B any](fn func(A) B) Caster[A, B] {
	return CasterFunc[A, B](fn)
}

type CasterErrFunc[A any, B any] func(A) (B, error)

func (c CasterErrFunc[A, B]) CastErr(v A) (B, error) {
	return c(v)
}

func CasterErrFn[A any, B any](fn func(A) (B, error)) CasterErr[A, B] {
	return CasterErrFunc[A, B](fn)
}
