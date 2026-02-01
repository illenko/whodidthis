interface LoadingProps {
  message?: string
}

export function Loading({ message = 'Loading...' }: LoadingProps) {
  return <div className="text-gray-500">{message}</div>
}

interface EmptyStateProps {
  title: string
  description?: string
}

export function EmptyState({ title, description }: EmptyStateProps) {
  return (
    <div className="text-center py-12">
      <div className="text-gray-500 mb-4">{title}</div>
      {description && <div className="text-sm text-gray-400">{description}</div>}
    </div>
  )
}
