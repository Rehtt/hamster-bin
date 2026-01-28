import { useEffect, useRef, useState } from 'react';
import { Html5Qrcode } from 'html5-qrcode';
import { Button } from './ui/Button';
import { toast } from 'react-hot-toast';

interface QRScannerProps {
  onScan: (data: string) => void;
  onClose: () => void;
  autoStart?: boolean;
}

export default function QRScanner({ onScan, onClose: _, autoStart = false }: QRScannerProps) {
  const [isScanning, setIsScanning] = useState(false);
  const scannerRef = useRef<Html5Qrcode | null>(null);
  const [isMobile, setIsMobile] = useState(false);

  useEffect(() => {
    const checkMobile = () => {
      setIsMobile(window.innerWidth < 768);
    };
    checkMobile();
  }, []);

  useEffect(() => {
    if (autoStart && !isScanning) {
      startScanning();
    }
  }, [autoStart]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    return () => {
      if (scannerRef.current && isScanning) {
        scannerRef.current.stop().catch(console.error);
      }
    };
  }, [isScanning]);

  const startScanning = async () => {
    try {
      if (!scannerRef.current) {
        scannerRef.current = new Html5Qrcode("qr-reader");
      }
      setIsScanning(true);
      await scannerRef.current.start(
        { facingMode: "environment" },
        { fps: 10, qrbox: { width: 250, height: 250 } },
        (decodedText) => {
          onScan(decodedText);
          stopScanning();
        },
        () => {} // error callback
      );
    } catch (err) {
      console.error(err);
      toast.error('无法启动摄像头');
      setIsScanning(false);
    }
  };

  const stopScanning = async () => {
    if (scannerRef.current && isScanning) {
        try {
            await scannerRef.current.stop();
            setIsScanning(false);
        } catch (e) {
            console.error(e);
        }
    }
  };

  const handleFileUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files.length > 0) {
      const file = e.target.files[0];
      try {
        if (!scannerRef.current) {
          scannerRef.current = new Html5Qrcode("qr-reader");
        }
        const result = await scannerRef.current.scanFile(file, true);
        onScan(result);
      } catch (err) {
        toast.error('无法识别图片中的二维码');
      }
    }
  };

  return (
    <div className="space-y-4">
      <div id="qr-reader" className="w-full overflow-hidden rounded-lg border"></div>
      <div className="flex justify-center gap-2">
        {!isScanning ? (
          <Button onClick={startScanning}>开始扫描</Button>
        ) : (
          <Button variant="destructive" onClick={stopScanning}>停止扫描</Button>
        )}
        {!isMobile && (
          <div className="relative">
               <input
                  type="file"
                  className="absolute inset-0 w-full h-full opacity-0 cursor-pointer"
                  accept="image/*"
                  onChange={handleFileUpload}
                />
              <Button variant="secondary">上传图片</Button>
          </div>
        )}
      </div>
    </div>
  );
}
