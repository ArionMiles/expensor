import js from '@eslint/js'
import jsxA11y from 'eslint-plugin-jsx-a11y'
import react from 'eslint-plugin-react'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'
import globals from 'globals'
import tseslint from 'typescript-eslint'

const sourceFiles = ['src/**/*.{ts,tsx}']
const mixedExportFiles = [
  'src/App.tsx',
  'src/components/Combobox.tsx',
  'src/components/ThemeProvider.tsx',
  'src/contexts/DisplayContext.tsx',
  'src/i18n/I18nProvider.tsx',
  'src/pages/Dashboard.tsx',
  'src/pages/dashboard/DashboardSections.tsx',
  'src/pages/rules/RuleFormSupport.tsx',
  'src/pages/settings/GeneralSettings.tsx',
  'src/pages/transactions/TransactionsParts.tsx',
]

export default tseslint.config(
  { ignores: ['coverage/', 'dist/'] },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  {
    ...react.configs.flat.recommended,
    files: sourceFiles,
    languageOptions: {
      ...react.configs.flat.recommended.languageOptions,
      globals: globals.browser,
    },
    settings: {
      react: { version: 'detect' },
    },
  },
  {
    ...react.configs.flat['jsx-runtime'],
    files: sourceFiles,
  },
  {
    ...jsxA11y.flatConfigs.recommended,
    files: sourceFiles,
    languageOptions: {
      ...jsxA11y.flatConfigs.recommended.languageOptions,
      globals: globals.browser,
    },
    rules: {
      ...jsxA11y.flatConfigs.recommended.rules,
      'jsx-a11y/no-autofocus': 'off',
    },
  },
  {
    files: sourceFiles,
    plugins: {
      'react-hooks': reactHooks,
    },
    rules: {
      'react-hooks/exhaustive-deps': 'warn',
      'react-hooks/rules-of-hooks': 'error',
    },
  },
  {
    ...reactRefresh.configs.vite,
    files: sourceFiles,
    ignores: ['src/**/*.test.{ts,tsx}', ...mixedExportFiles],
  },
  {
    files: sourceFiles,
    rules: {
      '@typescript-eslint/no-unused-vars': [
        'error',
        { argsIgnorePattern: '^_', caughtErrorsIgnorePattern: '^_', varsIgnorePattern: '^_' },
      ],
      'no-alert': 'error',
      'no-restricted-syntax': [
        'error',
        {
          selector: "JSXOpeningElement[name.name='select']",
          message: 'Use InlineSelect or another shared styled control instead of <select>.',
        },
        {
          selector: "JSXOpeningElement[name.name='datalist']",
          message: 'Use the shared Combobox instead of <datalist>.',
        },
        {
          selector:
            "JSXOpeningElement[name.type='JSXIdentifier'][name.name=/^[a-z]/] > JSXAttribute[name.name.name='title']",
          message: 'Use the project tooltip pattern instead of a native title attribute.',
        },
      ],
    },
  },
)
