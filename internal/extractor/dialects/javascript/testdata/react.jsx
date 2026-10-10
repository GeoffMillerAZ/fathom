import React, { memo, forwardRef, useEffect } from 'react';
import styled from 'styled-components';
import { Icon } from './icon';

export function Header({ title }) {
  return <h1><Icon.Glyph />{title}</h1>;
}

export const Page = ({ children }) => (
  <main>
    <Header title="x" />
    {children}
  </main>
);

const Item = memo(({ text }) => <li>{text}</li>);

export const Input = forwardRef((props, ref) => <input ref={ref} {...props} />);

export const Title = styled.h1`
  font-weight: bold;
`;

export class Clock extends React.Component {
  render() {
    return <Item text="tick" />;
  }
}

export function useInterval(fn, ms) {
  useEffect(() => {
    const id = setInterval(fn, ms);
    return () => clearInterval(id);
  }, [fn, ms]);
}

export function Parse(text) {
  return JSON.parse(text);
}
