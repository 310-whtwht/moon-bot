package strategy

import "time"

// Data is information from outside the price feed (for example the CFTC
// positioning of speculators) that scripts read with data(name).
type Data interface {
	// At returns the latest value of the named series that was public at t,
	// and false when there is none (unknown name, or no value yet).
	At(name string, t time.Time) (float64, bool)
}

// dataUser is implemented by strategies that read Data.
type dataUser interface {
	setData(Data)
}

// WithData returns a definition whose strategies read d. Strategies that do
// not use outside data are unaffected; a nil d leaves the definition as is.
func (d Definition) WithData(data Data) Definition {
	if data == nil {
		return d
	}
	factory := d.Factory
	d.Factory = func(p Params) Strategy {
		s := factory(p)
		if u, ok := s.(dataUser); ok {
			u.setData(data)
		}
		return s
	}
	return d
}
