import { useCallback, useEffect, useRef, useState } from 'react';
import { Html5Qrcode, Html5QrcodeSupportedFormats } from 'html5-qrcode';
import type { Html5QrcodeCameraScanConfig } from 'html5-qrcode';
import { Button } from './ui/Button';
import { toast } from 'react-hot-toast';

interface QRScannerProps {
  onScan: (data: string) => void;
  onClose: () => void;
  autoStart?: boolean;
}

const SCANNER_ELEMENT_ID = 'qr-reader';

const createScanner = () =>
  new Html5Qrcode(SCANNER_ELEMENT_ID, {
    formatsToSupport: [Html5QrcodeSupportedFormats.QR_CODE],
    verbose: false,
  });

const getAdaptiveQrBox = (viewfinderWidth: number, viewfinderHeight: number) => {
  const minEdge = Math.min(viewfinderWidth, viewfinderHeight);
  const size = Math.floor(minEdge * 0.85);
  return { width: size, height: size };
};

const CAMERA_SCAN_CONFIG: Html5QrcodeCameraScanConfig = {
  fps: 15,
  qrbox: getAdaptiveQrBox,
  aspectRatio: 1.777778,
  disableFlip: false,
  videoConstraints: {
    facingMode: 'environment',
    width: { ideal: 1280 },
    height: { ideal: 720 },
  },
};

export default function QRScanner({ onScan, autoStart = false }: QRScannerProps) {
  const [isScanning, setIsScanning] = useState(false);
  const scannerRef = useRef<Html5Qrcode | null>(null);
  const scanHandledRef = useRef(false);
  const [isMobile, setIsMobile] = useState(false);

  const ensureScanner = useCallback(() => {
    if (!scannerRef.current) {
      scannerRef.current = createScanner();
    }
    return scannerRef.current;
  }, []);

  const stopScanning = useCallback(async () => {
    const scanner = scannerRef.current;
    if (!scanner) {
      setIsScanning(false);
      scanHandledRef.current = false;
      return;
    }

    try {
      if (scanner.isScanning) {
        await scanner.stop();
      }
    } catch (e) {
      console.error(e);
    } finally {
      setIsScanning(false);
      scanHandledRef.current = false;
    }
  }, []);

  const handleScanSuccess = useCallback(
    (decodedText: string) => {
      if (scanHandledRef.current) return;
      scanHandledRef.current = true;
      onScan(decodedText);
      void stopScanning();
    },
    [onScan, stopScanning]
  );

  const startScanning = useCallback(async () => {
    if (scannerRef.current?.isScanning) return;

    scanHandledRef.current = false;
    try {
      const scanner = ensureScanner();
      setIsScanning(true);
      await scanner.start(
        { facingMode: 'environment' },
        CAMERA_SCAN_CONFIG,
        handleScanSuccess,
        () => {}
      );
    } catch (err) {
      console.error(err);
      toast.error('无法启动摄像头');
      setIsScanning(false);
      scanHandledRef.current = false;
    }
  }, [ensureScanner, handleScanSuccess]);

  useEffect(() => {
    const checkMobile = () => {
      setIsMobile(window.innerWidth < 768);
    };
    checkMobile();
  }, []);

  useEffect(() => {
    if (autoStart && !isScanning) {
      const startTimer = window.setTimeout(() => {
        void startScanning();
      }, 0);

      return () => window.clearTimeout(startTimer);
    }
  }, [autoStart, isScanning, startScanning]);

  useEffect(() => {
    return () => {
      const scanner = scannerRef.current;
      if (scanner?.isScanning) {
        scanner.stop().catch(console.error);
      }
    };
  }, []);

  const handleFileUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files.length > 0) {
      const file = e.target.files[0];
      try {
        const scanner = ensureScanner();
        const result = await scanner.scanFile(file, true);
        if (scanHandledRef.current) return;
        scanHandledRef.current = true;
        onScan(result);
      } catch {
        toast.error('无法识别图片中的二维码');
      } finally {
        e.target.value = '';
      }
    }
  };

  return (
    <div className="space-y-4">
      <div id={SCANNER_ELEMENT_ID} className="w-full overflow-hidden rounded-lg border"></div>
      <div className="flex justify-center gap-2">
        {!isScanning ? (
          <Button onClick={() => void startScanning()}>开始扫描</Button>
        ) : (
          <Button variant="destructive" onClick={() => void stopScanning()}>停止扫描</Button>
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
