import { Button } from './ui/Button';
import { useAuth } from '../context/useAuth';

export default function AuthError() {
  const { error, refresh } = useAuth();
  return (
    <div className="min-h-screen flex flex-col items-center justify-center gap-4 bg-background p-6">
      <p role="alert" className="text-destructive">{error}</p>
      <Button onClick={() => void refresh()}>重试</Button>
    </div>
  );
}
