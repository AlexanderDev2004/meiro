package m3

import (
	"math"

	"github.com/egoist/mygo/ui"
)

// animation is the token as MyGo's motion core sees it: a physical spring
// with the token's stiffness and damping, so a target that changes on the
// way keeps its velocity instead of starting over, and the desktop's
// reduced-motion preference cuts the motion short.
func (s Spring) animation() ui.Animation {
	return ui.Animation{Spring: ui.SpringOptions{
		Mass:      1,
		Stiffness: s.Stiffness,
		Damping:   2 * s.Damping * math.Sqrt(s.Stiffness),
	}}
}

// Motion is the token as a transition: changes ride the token's spring and
// keep their velocity when they are interrupted.
func (s Spring) Motion() ui.ElementTransition {
	a := s.animation()
	return ui.ElementTransition{Animation: &a}
}

// AnimateMotion eases a value of an element to its target along the token's
// spring, keeping its velocity when the target changes on the way.
func AnimateMotion(e ui.Element, key any, target float32, s Spring) float32 {
	return e.AnimateTo(key, target, s.animation())
}

// Rise is how far below its place, in DIPs, an element is while it comes in
// or goes out: from when entering is true, which is its first frame, and
// easing to to along the token's spring; 0 for an element in its place. The
// caller moves the element by it with its layout, Top or Bottom. A transform
// would do the same, but while any element of a frame is transformed the
// toolkit draws the clip of every rounded artwork on the CPU.
func Rise(e ui.Element, key any, from, to float32, entering bool, s Spring) float32 {
	if entering {
		e.Jump(key, from)
	}
	return AnimateMotion(e, key, to, s)
}
