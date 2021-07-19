// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import styled from 'styled-components';
import {useDispatch} from 'react-redux';

import {PlaybookRun} from 'src/types/playbook_run';

import Profile from 'src/components/profile/profile';

interface Props {
    userIds: string[];
}

const RHSParticipants: FC<Props> = (props: Props) => {
    return (
        <UserRow>
            {props.userIds.slice(0, 6).map((userId: string, idx: number) => (
                <UserPic
                    key={userId}
                    length={props.userIds.length}
                    idx={idx}
                >
                    <Profile
                        userId={userId}
                        withoutName={true}
                    />
                </UserPic>
            ))}
            {props.userIds.length > 6 &&
            <Rest>{'+' + (props.userIds.length - 6)}</Rest>
            }
        </UserRow>
    );
};

const UserRow = styled.div`
    padding: 0;
    display: flex;
    flex-direction: row;

    border-radius: 44px;

    :hover {
        outline: 6px solid rgba(var(--center-channel-color-rgb), 0.08);
        background-color: rgba(var(--center-channel-color-rgb), 0.08);
    }
`;

const UserPic = styled.div<{length: number, idx: number}>`
    .IncidentProfile {
        flex-direction: column;

        .name {
            display: none;
        }
    }

    && .image {
        margin: 0;
    }

    :not(:first-child) {
        margin-left: -8px;
    }

    position: relative;
    transition: transform .4s;

    z-index: ${(props) => props.length - props.idx};

    :hover {
        z-index: ${(props) => props.length};
    }

    && img {
        // We need both background-color and border color to imitate the color in the background
        background-color: var(--center-channel-bg);
        border: 2px solid var(--center-channel-color-04);
    }
`;

const Rest = styled.div`
    width: 32px;
    height: 32x;
    margin-left: -8px;
    border: 2px solid var(--center-channel-bg);
    border-radius: 50%;

    background-color: rgba(var(--center-channel-color-rgb), 0.16);
    color: rgba(var(--center-channel-color-rgb), 0.72);

    font-weight: 600;
    font-size: 11;

    display: flex;
    align-items: center;
    justify-content: center;
`;

export default RHSParticipants;