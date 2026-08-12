interface Props {
  tabId: string;
  title: string;
}

/** Stub for approval rule entry form (Wave 4 incomplete implementation). */
export function ApprovalRuleForm({ title }: Props) {
  return (
    <div className="tab-content">
      <h1>{title}</h1>
      <p className="text-muted">Approval rule entry form — placeholder pending backend API implementation.</p>
    </div>
  );
}
