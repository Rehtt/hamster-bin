import { useState } from 'react';
import { ImageOff } from 'lucide-react';
import { cn } from '../../utils/cn';
export function SafeImage({ src, alt = '', className }: { src?: string; alt?: string; className?: string }) {
  const [failed, setFailed] = useState('');
  return !src || failed === src ? <div role="img" aria-label={alt || '暂无图片'} className={cn('flex items-center justify-center rounded-md bg-muted text-muted-foreground', className)}><ImageOff className="size-5" /></div> : <img src={src} alt={alt} loading="lazy" onError={() => setFailed(src)} className={cn('rounded-md bg-muted object-contain', className)} />;
}
