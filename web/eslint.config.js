import js from '@eslint/js'
import tseslint from 'typescript-eslint'
import hooks from 'eslint-plugin-react-hooks'

const restrictedPrimitives=['button','input','select','textarea','table'].map(name=>({
  selector:`JSXOpeningElement[name.name='${name}']`,
  message:`Use the checked-in shadcn/ui ${name} primitive instead of raw <${name}>.`,
}))

export default tseslint.config(
  {ignores:['../internal/uiassets/dist']},
  js.configs.recommended,
  ...tseslint.configs.recommended,
  {files:['src/**/*.{ts,tsx}'],plugins:{'react-hooks':hooks},rules:{...hooks.configs.recommended.rules,'@typescript-eslint/no-explicit-any':'off'}},
  {files:['src/**/*.{ts,tsx}'],ignores:['src/components/ui/**'],rules:{'no-restricted-syntax':['error',...restrictedPrimitives]}},
)
