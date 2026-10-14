import { AlertCircle, CheckCircle2, Clock, Loader2, Square } from 'lucide-react';
import { testStatus } from '../lib/status';

const STYLES = {
  active: { className: 'bg-blue-100 text-blue-800', Icon: Loader2, spin: true },
  passed: { className: 'bg-green-100 text-green-800', Icon: CheckCircle2 },
  failed: { className: 'bg-red-100 text-red-800', Icon: AlertCircle },
  stopped: { className: 'bg-orange-100 text-orange-800', Icon: Square },
  finished: { className: 'bg-slate-100 text-slate-800', Icon: Clock },
};

const StatusBadge = ({ test }) => {
  const { category, label, detail } = testStatus(test);
  const { className, Icon, spin } = STYLES[category];
  return (
    <span
      className={`inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-medium ${className}`}
      title={detail}
    >
      <Icon size={12} className={spin ? 'animate-spin' : undefined} /> {label}
    </span>
  );
};

export default StatusBadge;
