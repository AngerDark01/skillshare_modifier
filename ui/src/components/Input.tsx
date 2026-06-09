import { useId } from 'react';
import type { InputHTMLAttributes, TextareaHTMLAttributes } from 'react';

// Re-export split components for backward compatibility
export { Checkbox } from './Checkbox';
export { Select, type SelectOption } from './Select';

interface InputProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'size'> {
  label?: string;
  size?: 'sm' | 'md';
}

export function Input({ label, className = '', style, id, size = 'md', ...props }: InputProps) {
  const autoId = useId();
  const inputId = id ?? autoId;
  const sizeClass = size === 'sm' ? 'px-3 py-1.5' : 'px-4 py-2.5';
  const fontSize = size === 'sm' ? '0.85rem' : '1rem';

  return (
    <div>
      {label && (
        <label
          htmlFor={inputId}
          className="block text-base text-pencil-light mb-1"
        >
          {label}
        </label>
      )}
      <input
        id={inputId}
        className={`
          ss-input
          w-full ${sizeClass} bg-surface border-2 border-muted text-pencil
          placeholder:text-muted-dark
          hover:border-muted-dark
          focus:outline-none focus:border-pencil
          transition-all
          rounded-[var(--radius-md)]
          ${className}
        `}
        style={{
          fontSize,
          ...style,
        }}
        {...props}
      />
    </div>
  );
}

interface TextareaProps extends Omit<TextareaHTMLAttributes<HTMLTextAreaElement>, 'size'> {
  label?: string;
  size?: 'sm' | 'md';
}

export function Textarea({ label, className = '', style, id, size = 'md', ...props }: TextareaProps) {
  const autoId = useId();
  const inputId = id ?? autoId;
  const sizeClass = size === 'sm' ? 'px-3 py-1.5' : 'px-4 py-3';
  const fontSize = size === 'sm' ? '0.85rem' : '0.95rem';

  return (
    <div>
      {label && (
        <label
          htmlFor={inputId}
          className="block text-base text-pencil-light mb-1"
        >
          {label}
        </label>
      )}
      <textarea
        id={inputId}
        className={`
          ss-input
          w-full ${sizeClass} bg-surface border-2 border-muted text-pencil
          placeholder:text-muted-dark
          hover:border-muted-dark
          focus:outline-none focus:border-pencil
          transition-all resize-y
          rounded-[var(--radius-md)]
          ${className}
        `}
        style={{
          fontSize,
          ...style,
        }}
        {...props}
      />
    </div>
  );
}
