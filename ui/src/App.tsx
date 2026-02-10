import { BrowserRouter } from 'react-router-dom';
import { Layout } from './components/Layout';
import { WorkspaceProvider } from './workspace/context';

function App() {
  return (
    <BrowserRouter>
      <WorkspaceProvider>
        <Layout />
      </WorkspaceProvider>
    </BrowserRouter>
  );
}

export default App;
