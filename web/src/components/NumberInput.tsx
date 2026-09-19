"use client";

// Input angka format Indonesia: titik = pemisah ribuan ("10.000" -> 10000).
// Tampilan otomatis diformat saat mengetik. Hanya bilangan bulat ≥ 0 (Robux & Rupiah).
export const parseNum = (s: string) => {
  const digits = s.replace(/\D/g, "");
  return digits ? Number(digits) : 0;
};
export const fmtNum = (n: number) => (n ? n.toLocaleString("id-ID") : "");

type Props = Omit<React.InputHTMLAttributes<HTMLInputElement>, "value" | "onChange" | "type"> & {
  value: number;
  onValue: (n: number) => void;
};

export default function NumberInput({ value, onValue, ...rest }: Props) {
  return (
    <input
      {...rest}
      type="text"
      inputMode="numeric"
      autoComplete="off"
      value={fmtNum(value)}
      onChange={(e) => onValue(parseNum(e.target.value))}
    />
  );
}
