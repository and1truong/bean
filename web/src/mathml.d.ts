// @types/react does not declare MathML intrinsic elements yet; the formula
// renderer augments the closed set it emits. Element content stays literal —
// these tags accept children/text like any intrinsic element.
import 'react'

declare module 'react' {
  namespace JSX {
    type MathMLProps = React.DetailedHTMLProps<React.HTMLAttributes<MathMLElement>, MathMLElement>
    interface IntrinsicElements {
      math: MathMLProps & {display?: 'block' | 'inline'; xmlns?: string}
      mfrac: MathMLProps
      mi: MathMLProps & {mathvariant?: 'normal' | 'bold' | 'italic'}
      mn: MathMLProps
      mo: MathMLProps & {stretchy?: boolean | 'true' | 'false'}
      mroot: MathMLProps
      mrow: MathMLProps
      msqrt: MathMLProps
      msub: MathMLProps
      msup: MathMLProps
      mtext: MathMLProps
      munderover: MathMLProps
    }
  }
}
