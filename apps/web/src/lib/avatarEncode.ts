import { squareCrop } from "./avatar";

// Avatars are small: the server takes up to 120 KiB, Mumble shows them
// small too. JPEG on a plain background (transparent parts would turn
// black).
const SIZE = 128;
const MAX_BYTES = 120 << 10;

// A picture file as a square JPEG for the server (browser only).
export const encodeAvatar = async (file: Blob): Promise<Uint8Array> => {
  const image = await createImageBitmap(file);
  const { x, y, size } = squareCrop(image.width, image.height);
  const canvas = document.createElement("canvas");
  canvas.width = canvas.height = SIZE;
  const ctx = canvas.getContext("2d")!;
  ctx.fillStyle = "#2b2d31";
  ctx.fillRect(0, 0, SIZE, SIZE);
  ctx.drawImage(image, x, y, size, size, 0, 0, SIZE, SIZE);
  image.close();
  const blob = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, "image/jpeg", 0.85));
  if (!blob) throw new Error("cannot encode the picture");
  const bytes = new Uint8Array(await blob.arrayBuffer());
  if (bytes.length > MAX_BYTES) throw new Error("the picture is too large");
  return bytes;
};
