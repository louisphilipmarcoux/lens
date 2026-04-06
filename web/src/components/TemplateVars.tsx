import type { TemplateVariable } from "../types";

interface Props {
  variables: TemplateVariable[];
  onChange: (name: string, value: string) => void;
}

export default function TemplateVars({ variables, onChange }: Props) {
  if (variables.length === 0) return null;

  return (
    <div className="flex gap-4 mb-4 flex-wrap">
      {variables.map((v) => (
        <div key={v.name} className="flex items-center gap-2">
          <label className="text-xs font-medium text-gray-500 uppercase tracking-wide">
            {v.label}
          </label>
          <select
            value={v.selected}
            onChange={(e) => onChange(v.name, e.target.value)}
            className="text-sm border border-gray-300 rounded px-2 py-1 bg-white focus:outline-none focus:ring-2 focus:ring-lens-500"
          >
            <option value="">All</option>
            {v.values.map((val) => (
              <option key={val} value={val}>
                {val}
              </option>
            ))}
          </select>
        </div>
      ))}
    </div>
  );
}
