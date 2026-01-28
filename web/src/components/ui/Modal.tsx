import * as React from "react"
import { createPortal } from "react-dom"
import { X } from "lucide-react"
import { cn } from "../../utils/cn"
import { Button } from "./Button"

interface ModalProps {
  isOpen: boolean
  onClose: () => void
  title: string
  children: React.ReactNode
  footer?: React.ReactNode
  className?: string
}

export function Modal({ isOpen, onClose, title, children, footer, className }: ModalProps) {
  if (!isOpen) return null

  return createPortal(
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm p-4">
      <div className={cn("relative w-full max-w-lg rounded-lg border bg-background p-6 shadow-lg animate-in fade-in zoom-in duration-200", className)}>
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-lg font-semibold leading-none tracking-tight">{title}</h3>
          <Button variant="ghost" size="icon" onClick={onClose} className="h-8 w-8 rounded-full">
            <X className="h-4 w-4" />
          </Button>
        </div>
        <div className="mb-4">{children}</div>
        {footer && <div className="flex justify-end gap-2 mt-4">{footer}</div>}
      </div>
    </div>,
    document.body
  )
}
