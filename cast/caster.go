package cast

import "github.com/go-faster/jx"

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

type CasterCodecJX[A any, B any] interface {
	Caster[A, B]
	EncodeJx(enc *jx.Encoder, v B)
	DecodeJx(dec *jx.Decoder) (B, error)
}

type CasterCodecErrJX[A any, B any] interface {
	CasterErr[A, B]
	EncodeJx(enc *jx.Encoder, v B) error
	DecodeJx(dec *jx.Decoder) (B, error)
}

type CasterBi[A any, B any] interface {
	CastToPlain(v A) B
	CastToPb(v B) A
}

type CasterBiFunc[A any, B any] struct {
	ToPlain func(A) B
	ToPb    func(B) A
}

func (c CasterBiFunc[A, B]) CastToPlain(v A) B {
	return c.ToPlain(v)
}

func (c CasterBiFunc[A, B]) CastToPb(v B) A {
	return c.ToPb(v)
}

func CasterBiFn[A any, B any](toPlain func(A) B, toPb func(B) A) CasterBi[A, B] {
	return CasterBiFunc[A, B]{ToPlain: toPlain, ToPb: toPb}
}

type CasterBiErr[A any, B any] interface {
	CastToPlainErr(v A) (B, error)
	CastToPbErr(v B) (A, error)
}

type CasterBiErrFunc[A any, B any] struct {
	ToPlain func(A) (B, error)
	ToPb    func(B) (A, error)
}

func (c CasterBiErrFunc[A, B]) CastToPlainErr(v A) (B, error) {
	return c.ToPlain(v)
}

func (c CasterBiErrFunc[A, B]) CastToPbErr(v B) (A, error) {
	return c.ToPb(v)
}

func CasterBiErrFn[A any, B any](toPlain func(A) (B, error), toPb func(B) (A, error)) CasterBiErr[A, B] {
	return CasterBiErrFunc[A, B]{ToPlain: toPlain, ToPb: toPb}
}
