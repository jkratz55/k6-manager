import Editor, { loader } from '@monaco-editor/react';
import * as monaco from 'monaco-editor';
import EditorWorker from 'monaco-editor/esm/vs/editor/editor.worker?worker';
import TsWorker from 'monaco-editor/esm/vs/language/typescript/ts.worker?worker';

// Bundle Monaco with the app instead of letting @monaco-editor/react fetch it
// from a CDN at runtime, so the editor works in air-gapped clusters and under
// a strict Content-Security-Policy. Pages using this component are lazy-loaded
// so Monaco stays out of the main bundle.
self.MonacoEnvironment = {
  getWorker(_, label) {
    if (label === 'typescript' || label === 'javascript') return new TsWorker();
    return new EditorWorker();
  },
};
loader.config({ monaco });

const CodeEditor = (props) => <Editor defaultLanguage="javascript" {...props} />;

export default CodeEditor;
