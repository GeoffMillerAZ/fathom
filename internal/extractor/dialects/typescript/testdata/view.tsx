import React, { useState } from 'react';

interface Props {
  title: string;
}

export function View({ title }: Props) {
  const [open, setOpen] = useState(false);
  const toggle = () => setOpen(!open);
  return <section onClick={toggle}>{render(title)}</section>;
}

export class Frame extends React.Component<Props> {
  render() {
    return <View title={this.props.title} />;
  }
}
