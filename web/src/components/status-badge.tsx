interface StatusBadgeProps {
  state: string;
}

const stateStyles: Record<string, string> = {
  idle: "bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400",
  preparing:
    "bg-yellow-100 text-yellow-700 dark:bg-yellow-900 dark:text-yellow-300",
  ready: "bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300",
  live: "bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300",
  stopping: "bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300",
};

export default function StatusBadge({ state }: StatusBadgeProps) {
  const style = stateStyles[state] ?? stateStyles.idle;
  return (
    <span className={`px-3 py-1 rounded-full text-xs font-medium ${style}`}>
      {state.toUpperCase()}
    </span>
  );
}
