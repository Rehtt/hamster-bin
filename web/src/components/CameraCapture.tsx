import { useEffect, useRef, useState } from 'react';
import { Camera, RefreshCw, X } from 'lucide-react';
import { Button } from './ui/Button';

interface CameraCaptureProps {
  onCapture: (file: File) => void;
  onClose: () => void;
}

export default function CameraCapture({ onCapture, onClose }: CameraCaptureProps) {
  const videoRef = useRef<HTMLVideoElement>(null);
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const [error, setError] = useState('');
  const [attempt, setAttempt] = useState(0);
  const [stream, setStream] = useState<MediaStream | null>(null);
  const [facingMode, setFacingMode] = useState<'user' | 'environment'>('environment');

  useEffect(() => {
    let active = true;
    let currentStream: MediaStream | null = null;

    const openCamera = async () => {
      try {
        setError('');
        const newStream = await navigator.mediaDevices.getUserMedia({
          video: { facingMode: facingMode }
        });

        if (!active) {
          newStream.getTracks().forEach(track => track.stop());
          return;
        }

        currentStream = newStream;
        setStream(newStream);
        if (videoRef.current) {
          videoRef.current.srcObject = newStream;
        }
      } catch (error) {
        console.error(error);
        if (active) setError('无法访问摄像头。请允许浏览器使用摄像头，或返回选择照片上传。');
      }
    };

    void openCamera();

    return () => {
      active = false;
      currentStream?.getTracks().forEach(track => track.stop());
    };
  }, [facingMode, attempt]);

  const handleCapture = () => {
    if (videoRef.current && canvasRef.current) {
      const video = videoRef.current;
      const canvas = canvasRef.current;
      
      // 设置 canvas 尺寸与视频一致
      canvas.width = video.videoWidth;
      canvas.height = video.videoHeight;
      
      const context = canvas.getContext('2d');
      if (context) {
        // 绘制当前帧
        context.drawImage(video, 0, 0, canvas.width, canvas.height);
        
        // 转换为 Blob/File
        canvas.toBlob((blob) => {
          if (blob) {
            const file = new File([blob], `capture-${Date.now()}.jpg`, { type: 'image/jpeg' });
            onCapture(file);
            stopCamera();
          }
        }, 'image/jpeg', 0.8);
      }
    }
  };

  const stopCamera = () => {
    if (stream) {
      stream.getTracks().forEach(track => track.stop());
    }
  };

  const switchCamera = () => {
    setFacingMode(prev => prev === 'user' ? 'environment' : 'user');
  };

  return (
    <div className="flex flex-col items-center gap-4 p-4 bg-background h-full">
      {error && <div role="alert" className="w-full space-y-3 rounded-lg bg-destructive/5 p-4 text-sm"><p className="text-destructive">{error}</p><Button variant="outline" onClick={() => setAttempt(value => value + 1)}>重试摄像头</Button></div>}
      <div className="relative w-full max-w-md aspect-[3/4] bg-black rounded-lg overflow-hidden">
        <video 
          ref={videoRef} 
          autoPlay 
          playsInline 
          className="w-full h-full object-cover"
        />
        <canvas ref={canvasRef} className="hidden" />
      </div>
      
      <div className="flex gap-4 items-center">
        <Button variant="outline" size="icon" onClick={onClose} title="关闭">
            <X className="h-6 w-6" />
        </Button>
        <Button 
            size="icon" 
            className="h-16 w-16 rounded-full border-4 border-background ring-2 ring-primary" 
            disabled={!!error || !stream}
            onClick={handleCapture}
            title="拍摄"
        >
            <Camera className="h-8 w-8" />
        </Button>
        <Button variant="outline" size="icon" onClick={switchCamera} title="切换摄像头">
            <RefreshCw className="h-6 w-6" />
        </Button>
      </div>
    </div>
  );
}
