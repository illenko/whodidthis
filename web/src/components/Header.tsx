import { useTheme } from '../contexts/ThemeContext'
import { navigate } from '../lib/router'
import type { ScanStatus } from '../api'
import { Button } from './Button'

interface HeaderProps {
  scanStatus: ScanStatus | null
  onScan: () => void
}

export function Header({ scanStatus, onScan }: HeaderProps) {
  const { theme, toggleTheme } = useTheme()

  return (
    <header className="border-b border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-900">
      <div className="max-w-6xl mx-auto px-6 py-4 flex items-center justify-between">
        <button
          onClick={() => navigate({ page: 'scans' })}
          className="text-xl font-semibold text-gray-900 dark:text-gray-100 hover:text-gray-700 dark:hover:text-gray-300 transition-colors"
          aria-label="Go to home page"
        >
          🤔 WhoDidThis?
        </button>
        <div className="flex items-center gap-4">
          {scanStatus?.running && (
            <span className="text-sm text-gray-500 dark:text-gray-400 flex items-center gap-2">
              <span className="w-2 h-2 bg-green-500 rounded-full animate-pulse" aria-hidden="true" />
              <span aria-live="polite">{scanStatus.progress || 'Scanning...'}</span>
            </span>
          )}
          <button
            onClick={toggleTheme}
            className="p-2 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors focus:outline-none focus:ring-2 focus:ring-gray-300 dark:focus:ring-gray-600"
            aria-label={`Switch to ${theme === 'light' ? 'dark' : 'light'} mode`}
          >
            {theme === 'light' ? '🌙' : '☀️'}
          </button>
          <Button
            onClick={onScan}
            disabled={scanStatus?.running}
            aria-label="Trigger a new scan"
          >
            Run Scan
          </Button>
        </div>
      </div>
    </header>
  )
}
