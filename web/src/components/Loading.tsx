interface LoadingProps {
  message?: string
}

export function Loading({ message = 'Loading...' }: LoadingProps) {
  return (
    <div className="flex items-center justify-center py-12">
      <div className="flex items-center gap-3">
        <div className="w-5 h-5 border-2 border-gray-300 dark:border-gray-600 border-t-gray-900 dark:border-t-gray-100 rounded-full animate-spin" aria-hidden="true" />
        <span className="text-gray-500 dark:text-gray-400" role="status" aria-live="polite">{message}</span>
      </div>
    </div>
  )
}

interface EmptyStateProps {
  title: string
  description?: string
}

export function EmptyState({ title, description }: EmptyStateProps) {
  return (
    <div className="text-center py-12">
      <div className="text-gray-500 dark:text-gray-400 mb-4">{title}</div>
      {description && <div className="text-sm text-gray-400 dark:text-gray-500">{description}</div>}
    </div>
  )
}
