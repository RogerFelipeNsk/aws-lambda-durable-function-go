import { QRCodeSVG } from "qrcode.react";

export default function PixQR({ payload }: { payload: string }) {
  return (
    <div className="pix">
      <QRCodeSVG value={payload} size={220} />
      <code>{payload}</code>
    </div>
  );
}
