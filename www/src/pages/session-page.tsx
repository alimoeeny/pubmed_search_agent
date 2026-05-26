import { useParams } from 'react-router-dom'

export function SessionPage() {
  const { id } = useParams<{ id: string }>()
  return <div className="p-8">Session {id} — coming soon</div>
}
