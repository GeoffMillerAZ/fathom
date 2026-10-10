import React, { memo, forwardRef, lazy, useState, PureComponent } from 'react';
import styled from 'styled-components';
import * as Primitive from '@radix-ui/react-dialog';
import { Icon } from './icon';

export interface ButtonProps {
  label: string;
  onClick?: () => void;
}

type CardProps = { title: string; body?: string };

type LinkProps = React.AnchorHTMLAttributes<HTMLAnchorElement> & { href: string; external?: boolean };

interface Theme {
  color: string;
}

export function Button({ label, onClick }: ButtonProps) {
  return <button onClick={onClick}><Icon.Glyph name="x" />{label}</button>;
}

export const Card = ({ title, body }: CardProps) => (
  <section>
    <Button label={title} />
    <p>{body}</p>
  </section>
);

const Row = memo(function Row(props: { id: number }) {
  return <div>{props.id}</div>;
});

export const Link = forwardRef<HTMLAnchorElement, LinkProps>(({ href, external }, ref) => <a ref={ref} href={href} target={external ? '_blank' : undefined} />);

export const Fancy = React.memo(Card);

export const Wrapper = styled.div`
  color: red;
`;

export const Big = styled(Button)`
  font-size: 2em;
`;

export const Img = styled.img.attrs({
  src: '/banner.png',
})`
  width: 100%;
`;

const Lazy = lazy(() => import('./lazy'));

export const Dialog = Primitive.Root;

export class Frame extends React.Component<CardProps> {
  render() {
    return <Card title={this.props.title} />;
  }
}

export class Pure extends PureComponent {
  render() {
    return null;
  }
}

export class Store {
  get() {
    return 1;
  }
}

export function Format(value: number) {
  return value.toFixed(2);
}

export function useToggle(initial = false) {
  const [on, setOn] = useState(initial);
  return [on, () => setOn(!on)] as const;
}

export const useTheme = (): Theme => React.useContext(ThemeContext);

export function useless() {
  return 1;
}

export function userName(id: number) {
  return String(id);
}

const ThemeContext = React.createContext<Theme>({ color: 'black' });

export async function GET() {
  return new ImageResponse(<div>og</div>);
}

const DROP_HEAD = Primitive.DROP_HEAD;
