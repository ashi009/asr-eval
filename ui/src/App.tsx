import { BrowserRouter } from 'react-router-dom';
import { Layout } from './components/Layout';
import { WorkspaceProvider } from './workspace/context';

import { ThemeProvider } from './utils/theme';

function App() {
  return (
    <BrowserRouter>
      <WorkspaceProvider>
        <ThemeProvider defaultTheme="system" storageKey="vite-ui-theme">
          <Layout />
        </ThemeProvider>
      </WorkspaceProvider>
    </BrowserRouter>
  );
}

export default App;
