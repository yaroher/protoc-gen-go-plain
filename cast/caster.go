package cast

type Caster[A, B any] = func(A) B

type CasterErr[A, B any] = func(A) (B, error)

// IntoPtr возвращает указатель на переданное значение
func IntoPtr[T any](v T) *T {
	return &v
}
