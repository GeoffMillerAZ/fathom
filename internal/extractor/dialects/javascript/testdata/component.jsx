import React from 'react';

export function Button({ label, onClick }) {
  const handle = () => onClick(label);
  return <button onClick={handle}>{format(label)}</button>;
}

class Panel extends React.Component {
  render() {
    return <div>{this.props.children}</div>;
  }
}

export default Panel;
