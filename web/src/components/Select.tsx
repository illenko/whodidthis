import type { SelectHTMLAttributes, ReactNode } from 'react'

interface SelectProps extends SelectHTMLAttributes<HTMLSelectElement> {
  label?: string
  children: ReactNode
}

export function Select({ label, className = '', children, ...props }: SelectProps) {
  const selectClasses = [
    'px-3 py-2 border rounded-lg text-sm transition-colors',
    'bg-white dark:bg-gray-800',
    'border-gray-300 dark:border-gray-600',
    'text-gray-900 dark:text-gray-100',
    'focus:outline-none focus:ring-2 focus:ring-gray-300 dark:focus:ring-gray-600 focus:border-transparent',
    'disabled:opacity-50 disabled:cursor-not-allowed',
    className
  ].filter(Boolean).join(' ')

  if (label) {
    return (
      <div className="space-y-1">
        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300">
          {label}
        </label>
        <select className={selectClasses} {...props}>
          {children}
        </select>
      </div>
    )
  }

  return (
    <select className={selectClasses} {...props}>
      {children}
    </select>
  )
}
