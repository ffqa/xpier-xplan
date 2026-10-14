/** A row of mutually exclusive small buttons (language, schedule, levels). */
export function Segmented<T extends string>({
  options,
  value,
  onChange,
  disabled,
  className,
}: {
  options: { value: T; label: React.ReactNode }[];
  value: T;
  onChange: (value: T) => void;
  disabled?: boolean;
  className?: string;
}) {
  return (
    <div className={`flex w-fit rounded-md border p-0.5 ${className ?? ""}`}>
      {options.map((option) => (
        <button
          key={option.value}
          type="button"
          disabled={disabled}
          onClick={() => option.value !== value && onChange(option.value)}
          className={`rounded px-2.5 py-0.5 text-xs transition-colors disabled:opacity-50 ${value === option.value ? "bg-accent text-foreground" : "text-muted-foreground hover:text-foreground"}`}
        >
          {option.label}
        </button>
      ))}
    </div>
  );
}
