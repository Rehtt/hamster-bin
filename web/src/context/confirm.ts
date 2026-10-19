import { createContext, useContext } from 'react';
export type ConfirmOptions = { message: string; title?: string; confirmLabel?: string; destructive?: boolean };
export const ConfirmContext = createContext<(options: string | ConfirmOptions) => Promise<boolean>>(async () => false);
export const useConfirm = () => useContext(ConfirmContext);
