import { render, screen } from '@testing-library/react'
import Page from './page'

test('renders the name and the contract it speaks', () => {
  render(<Page />)
  expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('tourian')
  expect(screen.getByText('tourian.v1.VelocityCommand')).toBeInTheDocument()
})
