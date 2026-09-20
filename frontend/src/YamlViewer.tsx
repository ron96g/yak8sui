import Editor, { loader } from '@monaco-editor/react'
import * as monaco from 'monaco-editor'
import { fonts, space } from './theme'

loader.config({ monaco })

export default function YamlViewer({
  value,
  readOnly = true,
  onChange,
}: {
  value: string
  readOnly?: boolean
  onChange?: (value: string) => void
}) {
  return (
    <Editor
      language="yaml"
      value={value}
      onChange={(next) => onChange?.(next ?? '')}
      theme="yak8sui-deep-space"
      beforeMount={(editor) => {
        editor.editor.defineTheme('yak8sui-deep-space', {
          base: 'vs-dark',
          inherit: true,
          rules: [
            { token: 'string', foreground: 'A5B4FC' },
            { token: 'number', foreground: '34D399' },
            { token: 'keyword', foreground: '38BDF8' },
            { token: 'type', foreground: '38BDF8' },
            { token: 'comment', foreground: 'A8B8CC', fontStyle: 'italic' },
          ],
          colors: {
            'editor.background': space.canvas,
            'editor.foreground': space.text,
            'editorLineNumber.foreground': space.outline,
            'editorLineNumber.activeForeground': space.muted,
            'editor.selectionBackground': '285D78',
            'editor.inactiveSelectionBackground': '203249',
            'editor.lineHighlightBackground': space.panel,
            'editorCursor.foreground': space.primary,
            'editorIndentGuide.background1': space.divider,
            'editorIndentGuide.activeBackground1': space.outline,
          },
        })
      }}
      options={{
        readOnly,
        domReadOnly: readOnly,
        minimap: { enabled: false },
        scrollBeyondLastLine: false,
        fontFamily: fonts.technical,
        fontSize: 14,
        lineHeight: 21,
        padding: { top: 12, bottom: 12 },
        renderLineHighlight: 'line',
        selectionHighlight: true,
      }}
    />
  )
}
