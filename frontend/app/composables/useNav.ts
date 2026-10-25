// the side menu on phones: a drawer that the top bar opens, closed by a tap outside, Escape or a page change
export function useNav() {
  const open = useState('nav-open', () => false)
  return { open }
}
